package agentmod

import (
	"testing"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

func mod(ref, desired string, direct bool) v2.AgentModSchema {
	return v2.AgentModSchema{ModReference: ref, DesiredVersion: desired, Direct: direct}
}

func lock(ref, version string, direct bool) v2.ModLock {
	return v2.ModLock{ModReference: ref, Version: version, Direct: direct}
}

func TestDiffReportsAddedMods(t *testing.T) {
	current := []v2.AgentModSchema{mod("RefinedPower", "3.3.0", true)}
	next := v2.Lockfile{Mods: []v2.ModLock{
		lock("RefinedPower", "3.3.0", true),
		lock("Ficsit", "1.0.0", false),
	}}

	c := Diff(current, next)

	if len(c.Added) != 1 || c.Added[0].ModReference != "Ficsit" {
		t.Fatalf("expected Ficsit to be added, got %+v", c.Added)
	}
	if !c.Added[0].Dependency {
		t.Fatal("expected a mod the resolver pulled in to be flagged as a dependency")
	}
	if len(c.Removed) != 0 || len(c.Changed) != 0 {
		t.Fatalf("expected nothing else to move, got %+v", c)
	}
}

func TestDiffReportsRemovedMods(t *testing.T) {
	current := []v2.AgentModSchema{
		mod("RefinedPower", "3.3.0", true),
		mod("Ficsit", "1.0.0", false),
	}
	next := v2.Lockfile{Mods: []v2.ModLock{lock("Ficsit", "1.0.0", true)}}

	c := Diff(current, next)

	if len(c.Removed) != 1 || c.Removed[0].ModReference != "RefinedPower" {
		t.Fatalf("expected RefinedPower to be removed, got %+v", c.Removed)
	}
}

func TestDiffReportsVersionChanges(t *testing.T) {
	current := []v2.AgentModSchema{mod("RefinedPower", "3.2.1", true)}
	next := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", true)}}

	c := Diff(current, next)

	if len(c.Changed) != 1 {
		t.Fatalf("expected one version change, got %+v", c.Changed)
	}
	if c.Changed[0].From != "3.2.1" || c.Changed[0].To != "3.3.0" {
		t.Fatalf("expected 3.2.1 -> 3.3.0, got %+v", c.Changed[0])
	}
	if len(c.Added) != 0 || len(c.Removed) != 0 {
		t.Fatal("a version change is not an add and a remove")
	}
}

// This is the safety property the old code lacked: a dependency two direct mods
// share must survive the removal of one of them. The resolver decides that by
// still emitting it, and Diff must not invent a removal.
func TestDiffKeepsASharedDependency(t *testing.T) {
	current := []v2.AgentModSchema{
		mod("RefinedPower", "3.3.0", true),
		mod("RefinedRD", "1.0.0", true),
		mod("Ficsit", "1.0.0", false),
	}
	// The user removed RefinedPower; the resolver still needs Ficsit for RefinedRD.
	next := v2.Lockfile{Mods: []v2.ModLock{
		lock("RefinedRD", "1.0.0", true),
		lock("Ficsit", "1.0.0", false),
	}}

	c := Diff(current, next)

	if len(c.Removed) != 1 || c.Removed[0].ModReference != "RefinedPower" {
		t.Fatalf("expected only RefinedPower removed, got %+v", c.Removed)
	}
}

func TestDiffIsEmptyWhenNothingMoves(t *testing.T) {
	current := []v2.AgentModSchema{mod("RefinedPower", "3.3.0", true)}
	next := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", true)}}

	if !Diff(current, next).IsEmpty() {
		t.Fatal("expected an unchanged selection to produce no change")
	}
}
