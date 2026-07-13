package agenttask

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestOrphanGateUpdatePlainReleaseForOrdinaryAction covers the common case: an
// orphaned startsfserver (or anything but syncmods) just gets its gate
// dropped, with no other field added.
func TestOrphanGateUpdatePlainReleaseForOrdinaryAction(t *testing.T) {
	now := time.Now()
	update := orphanGateUpdate("startsfserver", now)

	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("expected an $unset clause, got %v", update)
	}
	if _, present := unset["dependsOn"]; !present {
		t.Fatalf("expected dependsOn to be unset, got %v", unset)
	}

	set, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf("expected a $set clause, got %v", update)
	}
	if _, present := set["requiresServerStopped"]; present {
		t.Fatalf("an ordinary orphan must not gain requiresServerStopped, got %v", set)
	}
	if set["updatedAt"] != now {
		t.Fatalf("expected updatedAt to be stamped, got %v", set)
	}
}

// TestOrphanGateUpdateSyncModsIsRegatedNotReleased is the test the whole task
// exists for. A syncmods whose stopsfserver parent vanished must never come
// out of this sweep with no gate at all: dropped straight to claimable, it can
// be picked up while the game server is still running and rewrite the Mods
// directory underneath a live game — the exact corruption this subsystem
// exists to prevent. It must instead pick up requiresServerStopped in the SAME
// update as the dependsOn $unset, so there is no intermediate state, crash or
// not, where the task is gated on nothing.
//
// Sanity check: reverting the syncmods branch in orphanGateUpdate to a plain
// release (dropping the requiresServerStopped $set) turns this test red.
func TestOrphanGateUpdateSyncModsIsRegatedNotReleased(t *testing.T) {
	now := time.Now()
	update := orphanGateUpdate("syncmods", now)

	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("expected an $unset clause, got %v", update)
	}
	if _, present := unset["dependsOn"]; !present {
		t.Fatalf("expected dependsOn to be unset, got %v", unset)
	}

	set, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf("expected a $set clause, got %v", update)
	}
	if requiresStopped, present := set["requiresServerStopped"]; !present || requiresStopped != true {
		t.Fatalf("expected a released syncmods to be re-gated with requiresServerStopped: true in the SAME update, got %v", set)
	}
	if set["updatedAt"] != now {
		t.Fatalf("expected updatedAt to be stamped, got %v", set)
	}
}
