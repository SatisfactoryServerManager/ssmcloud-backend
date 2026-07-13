package agentmod

import (
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBackfillDocMarksEveryMigratedModDirect(t *testing.T) {
	// The old array cannot distinguish a mod the user chose from a dependency
	// the old code never tracked, so every migrated row must be direct: true
	// or the next resolve would treat a user's own mod as removable.
	agentID, accountID, modID := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	now := time.Now()

	sm := legacySelectedMod{
		ModID:            modID,
		DesiredVersion:   "1.2.3",
		InstalledVersion: "1.2.2",
		Installed:        true,
		Config:           "{\"foo\":true}",
	}

	doc := backfillDoc(agentID, accountID, modID, "SomeModRef", sm, now)

	if !doc.Direct {
		t.Fatalf("expected migrated mod to be direct")
	}
	if doc.AgentID != agentID || doc.AccountID != accountID || doc.ModID != modID {
		t.Fatalf("expected ids to be carried through unchanged, got %+v", doc)
	}
	if doc.ModReference != "SomeModRef" {
		t.Fatalf("expected modReference SomeModRef, got %q", doc.ModReference)
	}
	if doc.DesiredVersion != "1.2.3" || doc.InstalledVersion != "1.2.2" || !doc.Installed || doc.Config != sm.Config {
		t.Fatalf("expected legacy fields to be carried through unchanged, got %+v", doc)
	}
}

func TestBackfillWritesSkipsUnsetWhenAnyModFailsToResolve(t *testing.T) {
	// A mod that fails to resolve (pruned from the catalogue, or a transient
	// error) must not silently truncate the migration to whatever did resolve:
	// the caller uses failed=true to leave modConfig in place so the agent is
	// retried on the next boot, rather than losing the unresolved mod forever.
	agentID, accountID := bson.NewObjectID(), bson.NewObjectID()
	okMod := bson.NewObjectID()
	badMod := bson.NewObjectID()
	now := time.Now()

	resolved := []resolvedMod{
		{sm: legacySelectedMod{ModID: okMod, DesiredVersion: "1.0.0"}, ref: "GoodRef"},
		{sm: legacySelectedMod{ModID: badMod, DesiredVersion: "2.0.0"}, err: errors.New("mod not in catalogue")},
	}

	writes, failed := backfillWrites(agentID, accountID, resolved, now)

	if !failed {
		t.Fatalf("expected failed=true when one mod could not resolve")
	}
	if len(writes) != 1 {
		t.Fatalf("expected the one resolvable mod to still be written, got %d writes", len(writes))
	}
}

func TestBackfillWritesAllResolveIsNotFailed(t *testing.T) {
	agentID, accountID := bson.NewObjectID(), bson.NewObjectID()
	modA, modB := bson.NewObjectID(), bson.NewObjectID()
	now := time.Now()

	resolved := []resolvedMod{
		{sm: legacySelectedMod{ModID: modA, DesiredVersion: "1.0.0"}, ref: "RefA"},
		{sm: legacySelectedMod{ModID: modB, DesiredVersion: "2.0.0"}, ref: "RefB"},
	}

	writes, failed := backfillWrites(agentID, accountID, resolved, now)

	if failed {
		t.Fatalf("expected failed=false when every mod resolves")
	}
	if len(writes) != 2 {
		t.Fatalf("expected both mods to be written, got %d writes", len(writes))
	}
}

func TestBackfillWritesEmptyWhenEveryModFails(t *testing.T) {
	// The degenerate case: every lookup fails, so there must be zero writes
	// AND failed=true - the caller must not unset modConfig with nothing migrated.
	agentID, accountID := bson.NewObjectID(), bson.NewObjectID()
	modA := bson.NewObjectID()
	now := time.Now()

	resolved := []resolvedMod{
		{sm: legacySelectedMod{ModID: modA, DesiredVersion: "1.0.0"}, err: errors.New("boom")},
	}

	writes, failed := backfillWrites(agentID, accountID, resolved, now)

	if !failed {
		t.Fatalf("expected failed=true")
	}
	if len(writes) != 0 {
		t.Fatalf("expected zero writes, got %d", len(writes))
	}
}

func TestBackfillDocPreservesUninstalledState(t *testing.T) {
	agentID, accountID, modID := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()

	sm := legacySelectedMod{ModID: modID, DesiredVersion: "1.0.0"}

	doc := backfillDoc(agentID, accountID, modID, "OtherRef", sm, time.Now())

	if doc.Installed {
		t.Fatalf("expected Installed to stay false when the legacy row had never reported installed")
	}
	if doc.InstalledVersion != "" {
		t.Fatalf("expected no installed version, got %q", doc.InstalledVersion)
	}
}
