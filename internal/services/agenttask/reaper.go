package agenttask

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ReapExpiredLeases returns abandoned tasks to the queue.
//
// The attempt was already spent at claim time, so a crash-looping agent is
// bounded by MaxAttempts rather than retrying forever.
func ReapExpiredLeases() error {
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
				"$set":   bson.M{"status": v2.TaskStatusCancelled, "finishedAt": now, "updatedAt": now, "lastError": "lease expired"},
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
