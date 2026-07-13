package agentmod

import (
	"errors"
	"testing"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// The agent's report is the only source of truth for what is on disk, and it must
// touch nothing else. Writing desiredVersion here would let an agent silently
// undo a user's pin.
func TestInstalledUpdateTouchesOnlyDiskState(t *testing.T) {
	u := installedUpdate(v2.InstalledMod{
		ModReference:     "RefinedPower",
		InstalledVersion: "3.3.0",
		Installed:        true,
	})

	set := u["$set"].(bson.M)

	if set["installed"] != true || set["installedVersion"] != "3.3.0" {
		t.Fatalf("expected the report to be written, got %v", set)
	}
	if _, ok := set["updatedAt"]; !ok {
		t.Fatal("expected updatedAt to be bumped")
	}

	for _, forbidden := range []string{"desiredVersion", "direct", "config", "needsUpdate", "latestVersion"} {
		if _, ok := set[forbidden]; ok {
			t.Fatalf("agent report must not write %q", forbidden)
		}
	}
}

func TestInstalledUpdateClearsVersionWhenNotInstalled(t *testing.T) {
	u := installedUpdate(v2.InstalledMod{ModReference: "RefinedPower", Installed: false})

	set := u["$set"].(bson.M)
	if set["installed"] != false || set["installedVersion"] != "" {
		t.Fatalf("expected a removed mod to report no version, got %v", set)
	}
}

// A nil keep list cannot express user intent - it typically means the caller
// swallowed an error resolving the lockfile. It must be refused rather than
// treated as "delete everything".
func TestDeleteAbsentFilterRefusesNilKeep(t *testing.T) {
	agentID := bson.NewObjectID()

	filter, err := deleteAbsentFilter(agentID, nil)

	if err == nil {
		t.Fatal("expected an error for a nil keep list, got nil")
	}
	if !errors.Is(err, errNilKeepList) {
		t.Fatalf("expected errNilKeepList, got %v", err)
	}
	if filter != nil {
		t.Fatalf("expected no filter to be built for a refused call, got %v", filter)
	}
}

// An explicit empty slice is the real "user removed their last mod" state and
// must delete every row for the agent - refusing it would make the last mod
// un-removable.
func TestDeleteAbsentFilterExplicitEmptyKeepMatchesAllRows(t *testing.T) {
	agentID := bson.NewObjectID()

	filter, err := deleteAbsentFilter(agentID, []string{})
	if err != nil {
		t.Fatalf("expected an explicit empty keep list to be accepted, got %v", err)
	}

	if filter["agentId"] != agentID {
		t.Fatalf("expected filter scoped to agent %v, got %v", agentID, filter["agentId"])
	}

	nin := filter["modReference"].(bson.M)["$nin"].([]string)
	if len(nin) != 0 {
		t.Fatalf("expected an empty $nin list, got %v", nin)
	}
}

func TestDeleteAbsentFilterNonEmptyKeepExcludesOnlyThoseReferences(t *testing.T) {
	agentID := bson.NewObjectID()
	keep := []string{"RefinedPower", "ModularUI"}

	filter, err := deleteAbsentFilter(agentID, keep)
	if err != nil {
		t.Fatalf("expected a non-empty keep list to be accepted, got %v", err)
	}

	nin := filter["modReference"].(bson.M)["$nin"].([]string)
	if len(nin) != 2 || nin[0] != "RefinedPower" || nin[1] != "ModularUI" {
		t.Fatalf("expected $nin to carry exactly the kept references, got %v", nin)
	}
}
