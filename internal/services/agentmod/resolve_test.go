package agentmod

import (
	"testing"

	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
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

// The resolver hands back a version normalised through semver's String(), not
// the raw string the catalogue stored. A catalogue entry written as "v1.2.3"
// or "1.2" must still be found when the resolver asks for "1.2.3" - a
// byte-for-byte comparison would reject both and produce a baffling
// "no version in the catalogue" error for a perfectly valid mod.
func TestServerTargetMatchesNonCanonicalCatalogueVersions(t *testing.T) {
	// resolvedVersion is what the resolver would hand back: a semver-canonical
	// string built from its parsed, normalised version. catalogueVersion is
	// what the catalogue happens to store for that same release - not
	// guaranteed canonical, since the resolver's own version parser accepts
	// "v1.2.3" and "1.2" (missing patch defaults to 0) too.
	cases := []struct {
		catalogueVersion string
		resolvedVersion  string
	}{
		{catalogueVersion: "v1.2.3", resolvedVersion: "1.2.3"},
		{catalogueVersion: "1.2", resolvedVersion: "1.2.0"},
	}

	for _, tc := range cases {
		dbMod := &models.ModSchema{
			ModReference: "RefinedPower",
			Versions: []models.ModVersion{
				{
					Version: tc.catalogueVersion,
					Targets: []models.ModVersionTarget{
						{TargetName: "WindowsServer", Link: "/download"},
					},
				},
			},
		}

		target, err := serverTarget(dbMod, tc.resolvedVersion, "WindowsServer")
		if err != nil {
			t.Fatalf("catalogue version %q: expected a match for resolved version %q, got error: %v", tc.catalogueVersion, tc.resolvedVersion, err)
		}
		if target.Link != "/download" {
			t.Fatalf("catalogue version %q: got wrong target %+v", tc.catalogueVersion, target)
		}
	}
}

func TestServerTargetErrorsWhenPlatformHasNoBuild(t *testing.T) {
	dbMod := &models.ModSchema{
		ModReference: "RefinedPower",
		Versions: []models.ModVersion{
			{
				Version: "1.2.3",
				Targets: []models.ModVersionTarget{
					{TargetName: "LinuxServer", Link: "/download"},
				},
			},
		},
	}

	if _, err := serverTarget(dbMod, "1.2.3", "WindowsServer"); err == nil {
		t.Fatal("expected an error when the platform has no build, not a silent skip")
	}
}
