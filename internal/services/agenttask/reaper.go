package agenttask

import (
	"context"
	"time"

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
		if task.Attempts >= task.MaxAttempts {
			update = bson.M{
				"$set":   bson.M{"status": v2.TaskStatusDead, "finishedAt": now, "updatedAt": now, "lastError": "lease expired"},
				"$unset": bson.M{"active": "", "leaseToken": "", "leaseExpiresAt": ""},
			}
		} else {
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

		// Fence on the token we read, so we never clobber a task the agent just
		// renewed.
		fence := bson.M{"_id": task.ID, "leaseToken": task.LeaseToken, "status": v2.TaskStatusRunning}
		if _, err := collection().UpdateOne(ctx, fence, update); err != nil {
			return err
		}
	}

	return nil
}
