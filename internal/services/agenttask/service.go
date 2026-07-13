package agenttask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/google/uuid"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	collectionName = "agenttasks"
	modelName      = "AgentTask"

	LeaseDuration = 60 * time.Second
	finishedTTL   = 7 * 24 * time.Hour
)

func collection() *mongo.Collection {
	return repositories.GetMongoClient().GetCollection(collectionName)
}

func model() (*mongoose.Model, error) {
	return repositories.GetMongoClient().GetModel(modelName)
}

// EnsureIndexes creates the indexes the queue's correctness depends on.
//
// The two unique partial indexes are not optimizations. `uniq_running_per_agent`
// is what serializes tasks on an agent; `uniq_active_dedupe` is what makes an
// enqueue idempotent. Removing either reintroduces the double-install bug.
func EnsureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	col := collection()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}, {Key: "status", Value: 1}, {Key: "nextAttemptAt", Value: 1}},
			Options: options.Index().SetName("dispatch_claim"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "leaseExpiresAt", Value: 1}},
			Options: options.Index().SetName("reaper_sweep"),
		},
		{
			Keys: bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().
				SetName("uniq_running_per_agent").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"status": "running"}),
		},
		{
			// dedupeKey must be in the filter, not just the key. An empty key is
			// omitempty'd away, and a unique index reads a missing field as null,
			// so every un-deduped task on one agent would collide with the last.
			// Requiring the field to exist keeps "" meaning "never dedupe".
			Keys: bson.D{{Key: "agentId", Value: 1}, {Key: "dedupeKey", Value: 1}},
			Options: options.Index().
				SetName("uniq_active_dedupe").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{
					"active":    bson.M{"$exists": true},
					"dedupeKey": bson.M{"$exists": true},
				}),
		},
		{
			Keys: bson.D{{Key: "finishedAt", Value: 1}},
			Options: options.Index().
				SetName("finished_ttl").
				SetExpireAfterSeconds(int32(finishedTTL.Seconds())),
		},
	}

	if _, err := col.Indexes().CreateMany(ctx, indexes); err != nil {
		return err
	}

	logger.GetDebugLogger().Println("Ensured agenttasks indexes")
	return nil
}

// EnqueueOpts carries the optional gates through to the stored task. Zero
// value means "claimable as soon as the agent is idle", which is what every
// existing caller wants.
type EnqueueOpts struct {
	DependsOn             *bson.ObjectID
	RequiresServerStopped bool
}

// Enqueue creates a pending task. It is idempotent on dedupeKey: if an active
// task with the same key already exists, its id is returned instead of an error.
// This closes the window where a workflow step writes a task, crashes before
// persisting the id, and re-enqueues on restart.
func Enqueue(agentID, accountID bson.ObjectID, action string, data interface{}, dedupeKey string, trigger v2.TaskTrigger, opts EnqueueOpts) (string, error) {
	payload := ""
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return "", fmt.Errorf("error marshalling task data: %w", err)
		}
		payload = string(b)
	}

	doc := v2.NewAgentTaskDoc(agentID, accountID, action, payload, dedupeKey, trigger, v2.AgentTaskOpts{
		DependsOn:             opts.DependsOn,
		RequiresServerStopped: opts.RequiresServerStopped,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection().InsertOne(ctx, doc)
	if err == nil {
		notifyEnqueued(agentID)
		return doc.ID.Hex(), nil
	}

	if !mongo.IsDuplicateKeyError(err) || dedupeKey == "" {
		return "", err
	}

	// Someone beat us to it. Adopt the existing task.
	existing := &v2.AgentTaskSchema{}
	filter := bson.M{
		"agentId":   agentID,
		"dedupeKey": dedupeKey,
		"active":    bson.M{"$exists": true},
	}
	if err := collection().FindOne(ctx, filter).Decode(existing); err != nil {
		return "", fmt.Errorf("duplicate dedupeKey %q but no active task found: %w", dedupeKey, err)
	}

	return existing.ID.Hex(), nil
}

// ReplacePendingPayload overwrites the data of an already-pending task of the
// given action, returning its id, or "" if there was none. It marshals the
// payload the same way Enqueue does.
//
// This is the only safe way to update a not-yet-claimed task's payload: it
// matches status=pending only, never running, because a running task's
// payload is already in the agent's hands and rewriting it here would not
// reach the agent. Callers that need "replace if pending, otherwise enqueue"
// must not fall back to Enqueue's dedupe-adoption on failure to find a
// pending task, since adoption also matches a RUNNING task and would ship it
// a payload update it can never see.
func ReplacePendingPayload(agentID bson.ObjectID, action string, data interface{}) (string, error) {
	payload := ""
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return "", fmt.Errorf("error marshalling task data: %w", err)
		}
		payload = string(b)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	task := &v2.AgentTaskSchema{}
	err := collection().FindOneAndUpdate(ctx,
		bson.M{"agentId": agentID, "action": action, "status": v2.TaskStatusPending},
		bson.M{"$set": bson.M{"data": payload, "updatedAt": time.Now()}}).
		Decode(task)
	if err != nil {
		// Only "no pending task" may be swallowed. Any other error (timeout,
		// decode failure, ...) must propagate: returning ("", nil) here would
		// send the caller on to Enqueue, which can adopt a still-pending OR
		// still-running task on its dedupe key and ship it a stale payload.
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", nil
		}
		return "", err
	}

	logger.GetDebugLogger().Printf("replaced the payload on pending %s task %s", action, task.ID.Hex())
	return task.ID.Hex(), nil
}

// RegateForChain re-points an already-enqueued task at a new parent, replacing
// whatever gate it had before.
//
// It exists because Enqueue's dedupe-adoption path ignores the opts passed to
// it: when an enqueue lands on an existing active task via dedupeKey, that task
// keeps the gating it was created with. A caller that escalates a deferred
// syncmods (gated on requiresServerStopped) into a stopsfserver -> syncmods
// chain must therefore fix the gate up afterwards, or the adopted task stays
// requiresServerStopped-gated forever while the server it is waiting to stop
// is the very server the new stopsfserver task is about to stop — a deadlock.
func RegateForChain(taskID string, parent bson.ObjectID) error {
	oid, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = collection().UpdateOne(ctx,
		bson.M{"_id": oid},
		bson.M{
			"$set":   bson.M{"dependsOn": parent, "updatedAt": time.Now()},
			"$unset": bson.M{"requiresServerStopped": ""},
		})
	return err
}

// FindByDedupeKey returns the newest task with this key whatever its status, or
// nil if there is none.
//
// Enqueue can only adopt an *active* task, because that is all the unique index
// constrains. A caller that lost the id of a task which has since finished would
// therefore enqueue a duplicate and re-run work that already succeeded. Looking
// the task up by key, terminal or not, closes that window. Safe because the only
// keys in use are unique for all time (workflow id + action index).
func FindByDedupeKey(agentID bson.ObjectID, dedupeKey string) (*v2.AgentTaskSchema, error) {
	if dedupeKey == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})

	task := &v2.AgentTaskSchema{}
	err := collection().FindOne(ctx, bson.M{"agentId": agentID, "dedupeKey": dedupeKey}, opts).Decode(task)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

func Get(taskID string) (*v2.AgentTaskSchema, error) {
	oid, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return nil, err
	}

	m, err := model()
	if err != nil {
		return nil, err
	}

	task := &v2.AgentTaskSchema{}
	if err := m.FindOne(task, bson.M{"_id": oid}); err != nil {
		return nil, err
	}
	return task, nil
}

func ListForAgent(agentID bson.ObjectID, limit int64) ([]v2.AgentTaskSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(limit)
	cur, err := collection().Find(ctx, bson.M{"agentId": agentID}, opts)
	if err != nil {
		return nil, err
	}

	tasks := make([]v2.AgentTaskSchema, 0)
	if err := cur.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// Claim atomically moves the agent's oldest due task from pending to running and
// mints a fencing token. It returns (nil, nil) when nothing is due.
//
// If the agent already has a running task, `uniq_running_per_agent` rejects the
// write with E11000 and we report "busy" the same way as "nothing to do". Two
// replicas racing for the same agent therefore need no coordination: one wins,
// the other backs off.
func Claim(agentID bson.ObjectID, serverRunning bool) (*v2.AgentTaskSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	token := uuid.NewString()

	update := bson.M{
		"$set": bson.M{
			"status":         v2.TaskStatusRunning,
			"leaseToken":     token,
			"leaseExpiresAt": now.Add(LeaseDuration),
			"startedAt":      now,
			"updatedAt":      now,
		},
		"$inc": bson.M{"attempts": 1},
	}

	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "createdAt", Value: 1}}).
		SetReturnDocument(options.After)

	task := &v2.AgentTaskSchema{}
	err := collection().FindOneAndUpdate(ctx, claimFilter(agentID, now, serverRunning), update, opts).Decode(task)

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, nil // nothing due, or everything due is gated
	case mongo.IsDuplicateKeyError(err):
		return nil, nil // agent already has a running task
	case err != nil:
		return nil, err
	}

	return task, nil
}

// fenced builds the filter every lease-holding write must use. A task whose
// leaseToken has moved on will match nothing, so a zombie agent's report is a
// no-op rather than a corruption.
func fenced(taskID, leaseToken string) (bson.M, error) {
	oid, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return nil, err
	}
	return bson.M{"_id": oid, "leaseToken": leaseToken, "status": v2.TaskStatusRunning}, nil
}

func Complete(taskID, leaseToken string) error {
	oid, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	filter, err := fenced(taskID, leaseToken)
	if err != nil {
		return err
	}

	now := time.Now()

	// message is the in-flight progress note ("installing"), so it must go with the
	// lease. Leaving it set makes a finished task read as though it were still
	// working.
	update := bson.M{
		"$set":   bson.M{"status": v2.TaskStatusCompleted, "finishedAt": now, "updatedAt": now, "progress": int32(100)},
		"$unset": bson.M{"active": "", "leaseToken": "", "leaseExpiresAt": "", "message": ""},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	task := &v2.AgentTaskSchema{}
	err = collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(task)
	if errors.Is(err, mongo.ErrNoDocuments) {
		logger.GetDebugLogger().Printf("Complete: stale lease for task %s, ignoring", taskID)
		return nil
	}
	if err != nil {
		return err
	}

	if err := cascadeChildren(ctx, oid, v2.TaskStatusCompleted, task.AgentID); err != nil {
		logger.GetErrorLogger().Printf("error cascading children of completed task %s: %s", taskID, err.Error())
	}
	return nil
}

// Fail decides between a retry and a death, and honours a pending cancellation.
// A cancelled task is terminal: it must never come back on the next tick.
func Fail(taskID, leaseToken, errMsg string) error {
	filter, err := fenced(taskID, leaseToken)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	current := &v2.AgentTaskSchema{}
	if err := collection().FindOne(ctx, filter).Decode(current); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			logger.GetDebugLogger().Printf("Fail: stale lease for task %s, ignoring", taskID)
			return nil
		}
		return err
	}

	now := time.Now()
	var update bson.M

	switch {
	case current.CancelRequested:
		update = bson.M{
			"$set":   bson.M{"status": v2.TaskStatusCancelled, "finishedAt": now, "updatedAt": now, "lastError": errMsg},
			"$unset": bson.M{"active": "", "leaseToken": "", "leaseExpiresAt": "", "message": ""},
		}
	case current.Attempts >= current.MaxAttempts:
		update = bson.M{
			"$set":   bson.M{"status": v2.TaskStatusDead, "finishedAt": now, "updatedAt": now, "lastError": errMsg},
			"$unset": bson.M{"active": "", "leaseToken": "", "leaseExpiresAt": "", "message": ""},
		}
	default:
		update = bson.M{
			"$set": bson.M{
				"status":        v2.TaskStatusPending,
				"nextAttemptAt": now.Add(BackoffFor(current.Attempts)),
				"updatedAt":     now,
				"lastError":     errMsg,
			},
			"$unset": bson.M{"leaseToken": "", "leaseExpiresAt": "", "startedAt": ""},
		}
	}

	res, err := collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// The filter is still fenced on leaseToken+status=running from the FindOne above.
	// If the reaper terminalised this task between our FindOne and this UpdateOne, the
	// leaseToken has moved on and res.MatchedCount is 0: our write landed on nothing,
	// so cascading here would kill the chain of a task we no longer hold. Only cascade
	// the write that actually took effect.
	if res.MatchedCount > 0 && (current.CancelRequested || current.Attempts >= current.MaxAttempts) {
		status := v2.TaskStatusDead
		if current.CancelRequested {
			status = v2.TaskStatusCancelled
		}
		if err := cascadeChildren(ctx, current.ID, status, current.AgentID); err != nil {
			logger.GetErrorLogger().Printf("error cascading children of failed task %s: %s", taskID, err.Error())
		}
	}
	return nil
}

// Release returns a task to the queue without spending an attempt. It is the
// graceful-shutdown path: the agent chose to stop, so it should not be punished.
func Release(taskID, leaseToken string) error {
	filter, err := fenced(taskID, leaseToken)
	if err != nil {
		return err
	}

	now := time.Now()
	update := bson.M{
		"$set":   bson.M{"status": v2.TaskStatusPending, "nextAttemptAt": now, "updatedAt": now},
		"$inc":   bson.M{"attempts": -1},
		"$unset": bson.M{"leaseToken": "", "leaseExpiresAt": "", "startedAt": ""},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := collection().UpdateOne(ctx, filter, update); err != nil {
		return err
	}
	return nil
}

// RenewLease extends the lease and doubles as the cancellation channel. ok=false
// means the agent has lost the task and must abandon it.
func RenewLease(taskID, leaseToken string) (bool, bool, error) {
	filter, err := fenced(taskID, leaseToken)
	if err != nil {
		return false, false, err
	}

	now := time.Now()
	update := bson.M{"$set": bson.M{"leaseExpiresAt": now.Add(LeaseDuration), "updatedAt": now}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	task := &v2.AgentTaskSchema{}
	err = collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(task)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}

	return true, task.CancelRequested, nil
}

func ReportProgress(taskID, leaseToken string, pct int32, msg string) error {
	filter, err := fenced(taskID, leaseToken)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	update := bson.M{"$set": bson.M{"progress": pct, "message": msg, "updatedAt": time.Now()}}
	_, err = collection().UpdateOne(ctx, filter, update)
	return err
}

// Cancel takes a pending task straight to terminal. A running task cannot be
// yanked from under the agent, so it is flagged instead; the agent picks the flag
// up on its next lease renewal and unwinds.
func Cancel(taskID string) error {
	oid, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	task := &v2.AgentTaskSchema{}
	err = collection().FindOneAndUpdate(ctx,
		bson.M{"_id": oid, "status": v2.TaskStatusPending},
		bson.M{
			"$set":   bson.M{"status": v2.TaskStatusCancelled, "finishedAt": now, "updatedAt": now},
			"$unset": bson.M{"active": ""},
		}, opts).Decode(task)
	if err == nil {
		// The recovery-exempt startsfserver release (see cascadeChildren) needs the
		// real agentID to wake the dispatcher; without it the wake is silently
		// dropped and the recovery start waits for the next tick instead.
		if err := cascadeChildren(ctx, oid, v2.TaskStatusCancelled, task.AgentID); err != nil {
			logger.GetErrorLogger().Printf("error cascading children of cancelled task %s: %s", taskID, err.Error())
		}
		return nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}

	res, err := collection().UpdateOne(ctx,
		bson.M{"_id": oid, "status": v2.TaskStatusRunning},
		bson.M{"$set": bson.M{"cancelRequested": true, "updatedAt": now}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("task %s is not cancellable", taskID)
	}
	return nil
}

// Retry resurrects a dead task with a fresh attempt budget. If an identical
// action is already active, uniq_active_dedupe rejects the write and the caller
// gets a usable message rather than a second install.
func Retry(taskID string) error {
	oid, err := bson.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	res, err := collection().UpdateOne(ctx,
		bson.M{"_id": oid, "status": v2.TaskStatusDead},
		bson.M{
			"$set": bson.M{
				"status":          v2.TaskStatusPending,
				"active":          true,
				"attempts":        0,
				"nextAttemptAt":   now,
				"cancelRequested": false,
				"updatedAt":       now,
			},
			"$unset": bson.M{"leaseToken": "", "leaseExpiresAt": "", "startedAt": "", "finishedAt": "", "lastError": ""},
		})

	if mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf("a task for this action is already queued")
	}
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("task %s is not dead, cannot retry", taskID)
	}

	task, err := Get(taskID)
	if err != nil {
		return err
	}
	notifyEnqueued(task.AgentID)
	return nil
}

// recoveryExemptAction is the one action a dead/cancelled cascade must never
// cancel. stopsfserver -> syncmods -> startsfserver takes the game server down
// to rewrite Mods and bring it back up; if the chain dies anywhere after the
// stop, cancelling the remaining startsfserver would strand the server down
// with nothing left to start it — the exact failure the cascade exists to
// prevent, just moved one step later. The startsfserver handler is contractually
// idempotent ("already running: complete, no-op"), so releasing it is safe even
// if the chain died before the stop ever ran: it simply completes as a no-op
// against a server that was never stopped. Do not remove this exemption as
// "dead code cleanup" — it is the fix for that stranding case.
const recoveryExemptAction = "startsfserver"

// cascadeWrites is the single, tested expression of the cascade rule: what
// should happen to the tasks gated behind a parent that just reached a
// terminal state. cascadeChildren does nothing but execute what this returns,
// so there is exactly one place the rule can drift from what is tested.
//
// Completed parents simply lift the gate on every child. Dead/cancelled
// parents cancel every gated child EXCEPT recoveryExemptAction, which is
// released instead so it becomes claimable and can bring the server back up
// — see the comment on that constant for why.
//
// ORDER MATTERS for the dead/cancelled case, and cascadeChildren executes
// these ordered: the recovery release must be write [0] and the cancellation
// of the rest must be write [1]. BulkWrite applies ordered writes in slice
// order, so a crash between them leaves the recovery start claimable (the
// server recovers) rather than leaving it gated behind a parent that will
// never unblock it again (the server stays down). Do not reorder this slice.
func cascadeWrites(parentID bson.ObjectID, parentStatus string, now time.Time) []mongo.WriteModel {
	if parentStatus == v2.TaskStatusCompleted {
		return []mongo.WriteModel{
			mongo.NewUpdateManyModel().
				SetFilter(bson.M{"dependsOn": parentID}).
				SetUpdate(bson.M{"$unset": bson.M{"dependsOn": ""}, "$set": bson.M{"updatedAt": now}}),
		}
	}

	return []mongo.WriteModel{
		// [0] MUST run first: release the recovery-exempt child.
		mongo.NewUpdateManyModel().
			SetFilter(bson.M{"dependsOn": parentID, "active": bson.M{"$exists": true}, "action": recoveryExemptAction}).
			SetUpdate(bson.M{"$unset": bson.M{"dependsOn": ""}, "$set": bson.M{"updatedAt": now}}),
		// [1] MUST run second: cancel everything else.
		mongo.NewUpdateManyModel().
			SetFilter(bson.M{"dependsOn": parentID, "active": bson.M{"$exists": true}, "action": bson.M{"$ne": recoveryExemptAction}}).
			SetUpdate(bson.M{
				"$set":   bson.M{"status": v2.TaskStatusCancelled, "finishedAt": now, "updatedAt": now, "lastError": "cancelled with its parent task"},
				"$unset": bson.M{"active": "", "dependsOn": "", "leaseToken": "", "leaseExpiresAt": "", "message": ""},
			}),
	}
}

// cascadeChildren resolves the tasks gated behind a task that has just reached
// a terminal state. It only executes cascadeWrites and wakes the dispatcher;
// the rule itself lives in exactly one place, cascadeWrites, so it can be
// unit-tested without a database.
func cascadeChildren(ctx context.Context, parentID bson.ObjectID, parentStatus string, agentID bson.ObjectID) error {
	writes := cascadeWrites(parentID, parentStatus, time.Now())

	// Ordered (the default): mongo.BulkWrite applies writes[0] before writes[1].
	// See the ordering comment on cascadeWrites — do not set unordered here.
	if _, err := collection().BulkWrite(ctx, writes); err != nil {
		return err
	}

	// A child may now be claimable (gate lifted or recovery start released).
	// Waking on every cascade, even one that matched nothing, is a harmless
	// no-op: notifyEnqueued only nudges a dispatcher that already owns the
	// agent's connection, and dispatchFor is a no-op when nothing is due.
	notifyEnqueued(agentID)
	return nil
}
