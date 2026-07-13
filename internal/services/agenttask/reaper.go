package agenttask

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// orphanGracePeriod is how stale a pending task's dependsOn must be before the
// sweep is willing to call its parent non-existent.
//
// It is a correctness guard, not a tuning knob. A caller may deliberately gate a
// pending task onto a PRE-ASSIGNED _id and insert that parent a moment later
// (agentmod's executePlan does exactly this, and must, or the dispatcher's
// oldest-first claim can run the pair backwards). During that window the gate
// legitimately points at no document. Sweeping on updatedAt age keeps the sweep
// off any gate young enough for its parent to still be in flight.
const orphanGracePeriod = 5 * time.Minute

// ReapExpiredLeases returns abandoned tasks to the queue, then releases tasks
// gated behind a parent that does not exist.
//
// The attempt was already spent at claim time, so a crash-looping agent is
// bounded by MaxAttempts rather than retrying forever.
func ReapExpiredLeases() error {
	if err := reapExpiredLeases(); err != nil {
		return err
	}
	return releaseOrphanedGates()
}

// syncModsAction mirrors agentmod.ActionSyncMods. It cannot be imported: agentmod
// already imports agenttask, and this package must stay free of a cycle back to
// it. Keep the two literals in sync if either ever changes.
const syncModsAction = "syncmods"

// orphanGateUpdate is the whole of releaseOrphanedGates's decision: given the
// action of an orphaned task, what update makes it safe to run again?
//
// It is pure so the decision is testable without a database, and it exists at
// all because "release" is not uniformly safe. For every action except
// syncmods, the orphaned task's own idempotent handler is the safety net
// (restarting an already-running server is a no-op), so unsetting dependsOn
// and leaving no other gate is correct.
//
// A syncmods is the one action for which that is corruption, not a no-op: it
// rewrites the agent's Mods directory unconditionally, and doing that while
// the game server is running destroys the install. Its stopsfserver parent
// existing was the ONLY thing keeping it un-claimable, so losing that parent
// must swap in requiresServerStopped rather than drop to no gate at all — the
// dispatcher then only claims it once the agent reports the server stopped,
// which preserves the user's queued mod change without ever making it
// claimable over a live game. The $unset and $set below must land in the same
// update: releasing now and re-gating a moment later would leave exactly the
// ungated window a crash in between is supposed to never produce.
func orphanGateUpdate(action string, now time.Time) bson.M {
	set := bson.M{"updatedAt": now}
	if action == syncModsAction {
		set["requiresServerStopped"] = true
	}
	return bson.M{"$unset": bson.M{"dependsOn": ""}, "$set": set}
}

// releaseOrphanedGates makes a pending task whose dependsOn resolves to no
// document claimable again.
//
// Nothing else can recover one. cascadeChildren only fires from a real parent
// row's terminal transition and reapExpiredLeases only touches RUNNING tasks, so
// a gate pointing at an _id that was never inserted (a pre-assigned parent whose
// insert failed, and whose compensating release ALSO failed) has no parent to
// cascade and no lease to expire: without this sweep the task — in practice the
// startsfserver that brings the user's game server back up — stays pending
// forever and the server stays down.
func releaseOrphanedGates() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cur, err := collection().Find(ctx, bson.M{
		"status":    v2.TaskStatusPending,
		"dependsOn": bson.M{"$exists": true},
		"updatedAt": bson.M{"$lt": time.Now().Add(-orphanGracePeriod)},
	})
	if err != nil {
		return err
	}

	gated := make([]v2.AgentTaskSchema, 0)
	if err := cur.All(ctx, &gated); err != nil {
		return err
	}
	if len(gated) == 0 {
		return nil
	}

	parentIDs := make([]bson.ObjectID, 0, len(gated))
	for idx := range gated {
		if gated[idx].DependsOn != nil {
			parentIDs = append(parentIDs, *gated[idx].DependsOn)
		}
	}

	// One $in over _id: the parents that DO exist. Anything left is orphaned by
	// definition, whatever status the survivors are in — a live parent will
	// cascade its own children and needs no help from here.
	pcur, err := collection().Find(ctx, bson.M{"_id": bson.M{"$in": parentIDs}},
		options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return err
	}

	var parents []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := pcur.All(ctx, &parents); err != nil {
		return err
	}

	alive := make(map[bson.ObjectID]struct{}, len(parents))
	for _, p := range parents {
		alive[p.ID] = struct{}{}
	}

	now := time.Now()
	for idx := range gated {
		task := &gated[idx]
		if task.DependsOn == nil {
			continue
		}
		if _, ok := alive[*task.DependsOn]; ok {
			continue
		}

		// Fenced on the gate we read: if the real parent completed and cascaded the
		// gate off in the meantime, this matches nothing rather than clobbering a
		// gate that has since been re-pointed at a newer parent.
		res, err := collection().UpdateOne(ctx,
			bson.M{"_id": task.ID, "status": v2.TaskStatusPending, "dependsOn": *task.DependsOn},
			orphanGateUpdate(task.Action, now))
		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			continue
		}

		logger.GetErrorLogger().Printf("released task %s: its dependsOn %s resolves to no task", task.ID.Hex(), task.DependsOn.Hex())
		notifyEnqueued(task.AgentID)
	}

	return nil
}

func reapExpiredLeases() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()

	filter := bson.M{
		"status":         v2.TaskStatusRunning,
		"leaseExpiresAt": bson.M{"$lt": now},
	}

	cur, err := collection().Find(ctx, filter)
	if err != nil {
		return err
	}

	expired := make([]v2.AgentTaskSchema, 0)
	if err := cur.All(ctx, &expired); err != nil {
		return err
	}

	for idx := range expired {
		task := &expired[idx]

		var update bson.M
		var terminalStatus string
		switch {
		case task.CancelRequested:
			// A cancelled task must never come back on the next tick: requeueing it to
			// pending here would re-dispatch a task the user explicitly cancelled, since
			// the agent that would have reported FAILED and hit this flag in Fail is gone.
			terminalStatus = v2.TaskStatusCancelled
			update = bson.M{
				"$set":   bson.M{"status": v2.TaskStatusCancelled, "finishedAt": now, "updatedAt": now, "lastError": "cancelled; the agent did not acknowledge before its lease expired"},
				"$unset": bson.M{"active": "", "leaseToken": "", "leaseExpiresAt": ""},
			}
		case task.Attempts >= task.MaxAttempts:
			terminalStatus = v2.TaskStatusDead
			update = bson.M{
				"$set":   bson.M{"status": v2.TaskStatusDead, "finishedAt": now, "updatedAt": now, "lastError": "lease expired"},
				"$unset": bson.M{"active": "", "leaseToken": "", "leaseExpiresAt": ""},
			}
		default:
			update = bson.M{
				"$set": bson.M{
					"status":        v2.TaskStatusPending,
					"nextAttemptAt": now.Add(BackoffFor(task.Attempts)),
					"updatedAt":     now,
					"lastError":     "lease expired",
				},
				"$unset": bson.M{"leaseToken": "", "leaseExpiresAt": "", "startedAt": ""},
			}
		}

		// Fence on the token we read AND on the lease still being expired.
		//
		// The token alone is not enough: RenewLease does not rotate it, so an agent
		// that renews between our Find and this UpdateOne would still match the
		// filter and have its live task yanked back to pending. Re-checking
		// leaseExpiresAt closes that window — a renewal pushes the expiry into the
		// future and this write becomes a no-op.
		fence := bson.M{
			"_id":            task.ID,
			"leaseToken":     task.LeaseToken,
			"status":         v2.TaskStatusRunning,
			"leaseExpiresAt": bson.M{"$lt": now},
		}
		res, err := collection().UpdateOne(ctx, fence, update)
		if err != nil {
			return err
		}

		// A task that won the renewal race in the fencing window is still alive:
		// cascading its children away here would sever a chain that has not actually
		// died. Only cascade the write that actually took effect.
		if terminalStatus != "" && res.MatchedCount > 0 {
			if err := cascadeChildren(ctx, task.ID, terminalStatus, task.AgentID); err != nil {
				logger.GetErrorLogger().Printf("error cascading children of reaped task %s: %s", task.ID.Hex(), err.Error())
			}
		}
	}

	return nil
}
