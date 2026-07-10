package agenttask

import (
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	baseBackoff = 5 * time.Second
	maxBackoff  = 5 * time.Minute
)

// BackoffFor returns the delay before a task that has consumed `attempts`
// attempts may be claimed again: 5s, 10s, 20s, 40s, ... capped at 5m.
func BackoffFor(attempts int) time.Duration {
	if attempts < 1 {
		return baseBackoff
	}

	d := baseBackoff
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= maxBackoff {
			return maxBackoff
		}
	}
	return d
}

// WorkflowDedupeKey makes a workflow step's enqueue idempotent. Re-running the
// step produces the same key, and the unique partial index rejects the second
// insert.
func WorkflowDedupeKey(workflowID bson.ObjectID, actionIdx int) string {
	return fmt.Sprintf("workflow:%s:%d", workflowID.Hex(), actionIdx)
}

// BootUpdateDedupeKey scopes the UpdateOnStart task to one agent process, so a
// reconnect loop cannot queue a stack of update tasks while one is still active.
// It keys on the agent's session id, which survives reconnects, and not on the
// per-stream id.
func BootUpdateDedupeKey(sessionID string) string {
	return "boot-update:" + sessionID
}
