package agentmod

import (
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
