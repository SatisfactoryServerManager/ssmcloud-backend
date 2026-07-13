package agenttask

import (
	"time"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// claimFilter builds the predicate for the atomic claim.
//
// It is a pure function so the two gates can be tested without a database. Both
// gates are claim predicates and nothing more: dependsOn is cleared by the
// parent's terminal transition, and requiresServerStopped is evaluated against
// the agent status the state pipeline already maintains.
func claimFilter(agentID bson.ObjectID, now time.Time, serverRunning bool) bson.M {
	f := bson.M{
		"agentId":       agentID,
		"status":        v2.TaskStatusPending,
		"nextAttemptAt": bson.M{"$lte": now},
		"dependsOn":     bson.M{"$exists": false},
	}

	if serverRunning {
		f["requiresServerStopped"] = bson.M{"$ne": true}
	}

	return f
}
