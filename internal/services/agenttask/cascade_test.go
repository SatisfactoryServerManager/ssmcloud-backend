package agenttask

import (
	"testing"
	"time"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// updateManyFilter unwraps the private filter/update fields of an
// *mongo.UpdateManyModel via its exported accessors so the test can assert on
// exactly what will be sent to Mongo.
func updateManyFilter(t *testing.T, w mongo.WriteModel) bson.M {
	t.Helper()
	m, ok := w.(*mongo.UpdateManyModel)
	if !ok {
		t.Fatalf("expected *mongo.UpdateManyModel, got %T", w)
	}
	f, ok := m.Filter.(bson.M)
	if !ok {
		t.Fatalf("expected filter to be bson.M, got %T", m.Filter)
	}
	return f
}

func updateManyUpdate(t *testing.T, w mongo.WriteModel) bson.M {
	t.Helper()
	m, ok := w.(*mongo.UpdateManyModel)
	if !ok {
		t.Fatalf("expected *mongo.UpdateManyModel, got %T", w)
	}
	u, ok := m.Update.(bson.M)
	if !ok {
		t.Fatalf("expected update to be bson.M, got %T", m.Update)
	}
	return u
}

func TestCascadeWritesCompletedParentReleasesEveryChild(t *testing.T) {
	parentID := bson.NewObjectID()
	now := time.Now()

	writes := cascadeWrites(parentID, v2.TaskStatusCompleted, now)
	if len(writes) != 1 {
		t.Fatalf("expected exactly one write for a completed parent, got %d", len(writes))
	}

	filter := updateManyFilter(t, writes[0])
	if filter["dependsOn"] != parentID {
		t.Fatalf("expected the write to target children of %v, got filter %v", parentID, filter)
	}
	if _, exempted := filter["action"]; exempted {
		t.Fatalf("a completed parent must not exempt any action, got filter %v", filter)
	}

	update := updateManyUpdate(t, writes[0])
	unset, ok := update["$unset"].(bson.M)
	if !ok || unset["dependsOn"] != "" {
		t.Fatalf("expected the gate (dependsOn) to be lifted, got update %v", update)
	}
	if _, cancelled := update["$set"].(bson.M)["status"]; cancelled {
		t.Fatalf("a completed parent must not set a status on its children, got update %v", update)
	}
}

// This is the invariant the whole cascade design revolves around: a
// startsfserver child must survive its parent dying or being cancelled, or the
// user's game server can be left stopped with nothing left to bring it back
// up. It must be expressed as a hard exclusion in the cancellation write's
// filter — asserting on the actual bson.M here means deleting the exemption
// from cascadeWrites turns this test red, unlike the old decideCascade tests
// which stayed green even with the exemption removed from the real code path.
func TestCascadeWritesDeadOrCancelledParentExemptsRecoveryStart(t *testing.T) {
	for _, parentStatus := range []string{v2.TaskStatusDead, v2.TaskStatusCancelled} {
		parentID := bson.NewObjectID()
		now := time.Now()

		writes := cascadeWrites(parentID, parentStatus, now)
		if len(writes) != 2 {
			t.Fatalf("[%s] expected exactly two writes (release then cancel), got %d", parentStatus, len(writes))
		}

		// Write [0]: the recovery release, and it must run first.
		releaseFilter := updateManyFilter(t, writes[0])
		if releaseFilter["dependsOn"] != parentID {
			t.Fatalf("[%s] release write not scoped to this parent: %v", parentStatus, releaseFilter)
		}
		if releaseFilter["action"] != recoveryExemptAction {
			t.Fatalf("[%s] expected write[0] to target only %q, got filter %v", parentStatus, recoveryExemptAction, releaseFilter)
		}
		releaseUpdate := updateManyUpdate(t, writes[0])
		if unset, ok := releaseUpdate["$unset"].(bson.M); !ok || unset["dependsOn"] != "" {
			t.Fatalf("[%s] expected write[0] to clear dependsOn, got %v", parentStatus, releaseUpdate)
		}
		if set, ok := releaseUpdate["$set"].(bson.M); ok {
			if _, hasStatus := set["status"]; hasStatus {
				t.Fatalf("[%s] the recovery release must not set a terminal status, got %v", parentStatus, releaseUpdate)
			}
		}

		// Write [1]: cancels everyone else, and it must exclude the recovery action.
		cancelFilter := updateManyFilter(t, writes[1])
		if cancelFilter["dependsOn"] != parentID {
			t.Fatalf("[%s] cancel write not scoped to this parent: %v", parentStatus, cancelFilter)
		}
		actionExclusion, ok := cancelFilter["action"].(bson.M)
		if !ok || actionExclusion["$ne"] != recoveryExemptAction {
			t.Fatalf("[%s] expected the cancellation filter to exclude %q via $ne, got %v", parentStatus, recoveryExemptAction, cancelFilter)
		}
		cancelUpdate := updateManyUpdate(t, writes[1])
		set, ok := cancelUpdate["$set"].(bson.M)
		if !ok || set["status"] != v2.TaskStatusCancelled {
			t.Fatalf("[%s] expected the non-exempt children to be cancelled, got %v", parentStatus, cancelUpdate)
		}
	}
}

func TestCascadeWritesDeadOrCancelledParentCancelUpdateUnsetsLeaseFields(t *testing.T) {
	writes := cascadeWrites(bson.NewObjectID(), v2.TaskStatusDead, time.Now())
	update := updateManyUpdate(t, writes[1])
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("expected an $unset clause, got %v", update)
	}
	for _, field := range []string{"active", "dependsOn", "leaseToken", "leaseExpiresAt", "message"} {
		if _, present := unset[field]; !present {
			t.Fatalf("expected cancellation to unset %q, got %v", field, unset)
		}
	}
}
