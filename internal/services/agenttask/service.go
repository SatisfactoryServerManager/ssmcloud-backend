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

// Enqueue creates a pending task. It is idempotent on dedupeKey: if an active
// task with the same key already exists, its id is returned instead of an error.
// This closes the window where a workflow step writes a task, crashes before
// persisting the id, and re-enqueues on restart.
func Enqueue(agentID, accountID bson.ObjectID, action string, data interface{}, dedupeKey string, trigger v2.TaskTrigger) (string, error) {
	payload := ""
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return "", fmt.Errorf("error marshalling task data: %w", err)
		}
		payload = string(b)
	}

	doc := v2.NewAgentTaskDoc(agentID, accountID, action, payload, dedupeKey, trigger)

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
func Claim(agentID bson.ObjectID) (*v2.AgentTaskSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()
	token := uuid.NewString()

	filter := bson.M{
		"agentId":       agentID,
		"status":        v2.TaskStatusPending,
		"nextAttemptAt": bson.M{"$lte": now},
	}

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
	err := collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(task)

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		return nil, nil // nothing due
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

	res, err := collection().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		logger.GetDebugLogger().Printf("Complete: stale lease for task %s, ignoring", taskID)
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

	_, err = collection().UpdateOne(ctx, filter, update)
	return err
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

	res, err := collection().UpdateOne(ctx,
		bson.M{"_id": oid, "status": v2.TaskStatusPending},
		bson.M{
			"$set":   bson.M{"status": v2.TaskStatusCancelled, "finishedAt": now, "updatedAt": now},
			"$unset": bson.M{"active": ""},
		})
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 {
		return nil
	}

	res, err = collection().UpdateOne(ctx,
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
