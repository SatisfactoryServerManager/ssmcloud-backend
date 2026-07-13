package agentmod

import (
	"testing"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

func TestNextSelectionAddsADirectMod(t *testing.T) {
	current := []v2.AgentModSchema{mod("RefinedPower", "3.3.0", true)}

	next := nextSelection(current, ModChange{Op: OpAdd, ModReference: "RefinedRD"}, nil)

	if _, ok := next["RefinedRD"]; !ok {
		t.Fatalf("expected RefinedRD to be selected, got %v", next)
	}
	if next["RefinedRD"] != "" {
		t.Fatal("expected an added mod to be unpinned, so the resolver takes the latest compatible version")
	}
	if next["RefinedPower"] != "3.3.0" {
		t.Fatal("expected the existing pin to survive")
	}
}

// Only direct mods form the selection. A dependency is not something the user can
// remove directly; it leaves when nothing needs it.
func TestNextSelectionIgnoresDependencies(t *testing.T) {
	current := []v2.AgentModSchema{
		mod("RefinedPower", "3.3.0", true),
		mod("Ficsit", "1.0.0", false),
	}

	next := nextSelection(current, ModChange{Op: OpAdd, ModReference: "RefinedRD"}, nil)

	if _, ok := next["Ficsit"]; ok {
		t.Fatal("expected the dependency to be left to the resolver, not pinned as a direct choice")
	}
}

func TestNextSelectionRemovesADirectMod(t *testing.T) {
	current := []v2.AgentModSchema{
		mod("RefinedPower", "3.3.0", true),
		mod("RefinedRD", "1.0.0", true),
	}

	next := nextSelection(current, ModChange{Op: OpRemove, ModReference: "RefinedPower"}, nil)

	if _, ok := next["RefinedPower"]; ok {
		t.Fatal("expected RefinedPower to leave the selection")
	}
	if next["RefinedRD"] != "1.0.0" {
		t.Fatal("expected the other mod to be untouched")
	}
}

func TestNextSelectionSetsAVersion(t *testing.T) {
	current := []v2.AgentModSchema{mod("RefinedPower", "3.2.1", true)}

	next := nextSelection(current, ModChange{Op: OpSetVersion, ModReference: "RefinedPower", Version: "3.3.0"}, nil)

	if next["RefinedPower"] != "3.3.0" {
		t.Fatalf("expected the pin to move to 3.3.0, got %q", next["RefinedPower"])
	}
}

// "Update all" moves every direct mod's pin to the catalogue's latest. It must not
// touch a mod with no newer version, and must not promote a dependency.
func TestNextSelectionUpdateAllBumpsOnlyDirectModsWithUpdates(t *testing.T) {
	current := []v2.AgentModSchema{
		mod("RefinedPower", "3.2.1", true),
		mod("RefinedRD", "1.0.0", true),
		mod("Ficsit", "1.0.0", false),
	}
	latest := map[string]string{
		"RefinedPower": "3.3.0",
		"RefinedRD":    "1.0.0",
		"Ficsit":       "2.0.0",
	}

	next := nextSelection(current, ModChange{Op: OpUpdateAll}, latest)

	if next["RefinedPower"] != "3.3.0" {
		t.Fatalf("expected RefinedPower to be bumped, got %q", next["RefinedPower"])
	}
	if next["RefinedRD"] != "1.0.0" {
		t.Fatalf("expected RefinedRD to stay put, got %q", next["RefinedRD"])
	}
	if _, ok := next["Ficsit"]; ok {
		t.Fatal("expected update-all to leave dependencies to the resolver")
	}
}
