package agenttask

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestClaimFilterAlwaysExcludesGatedDependencies(t *testing.T) {
	f := claimFilter(bson.NewObjectID(), time.Now(), false)

	got, ok := f["dependsOn"]
	if !ok {
		t.Fatal("expected the claim to exclude tasks still waiting on a parent")
	}
	if got.(bson.M)["$exists"] != false {
		t.Fatalf("expected dependsOn: {$exists: false}, got %v", got)
	}
}

// The whole point of the gate: while the agent reports the server running, a
// requiresServerStopped task must not be claimable.
func TestClaimFilterExcludesServerStoppedTasksWhileRunning(t *testing.T) {
	f := claimFilter(bson.NewObjectID(), time.Now(), true)

	got, ok := f["requiresServerStopped"]
	if !ok {
		t.Fatal("expected the claim to exclude gated tasks while the server runs")
	}
	if got.(bson.M)["$ne"] != true {
		t.Fatalf("expected requiresServerStopped: {$ne: true}, got %v", got)
	}
}

func TestClaimFilterAllowsServerStoppedTasksOnceStopped(t *testing.T) {
	f := claimFilter(bson.NewObjectID(), time.Now(), false)

	if _, ok := f["requiresServerStopped"]; ok {
		t.Fatal("expected no gate on requiresServerStopped once the server is stopped")
	}
}

func TestClaimFilterKeepsTheExistingPredicates(t *testing.T) {
	agentID := bson.NewObjectID()
	now := time.Now()

	f := claimFilter(agentID, now, false)

	if f["agentId"] != agentID {
		t.Fatal("expected the claim to be scoped to the agent")
	}
	if f["status"] != "pending" {
		t.Fatal("expected the claim to take only pending tasks")
	}
	if f["nextAttemptAt"].(bson.M)["$lte"] != now {
		t.Fatal("expected the claim to honour the backoff schedule")
	}
}
