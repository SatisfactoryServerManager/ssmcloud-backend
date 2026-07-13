package agentmod

import (
	"testing"

	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
)

func TestNewestVersionPicksTheHighestSemver(t *testing.T) {
	// The catalogue's version order is not guaranteed, so this cannot just take
	// the first element — which is what the old CheckAgentModsConfigs did.
	got := newestVersion([]models.ModVersion{
		{Version: "3.2.1"},
		{Version: "3.10.0"},
		{Version: "3.3.0"},
	})

	if got != "3.10.0" {
		t.Fatalf("expected 3.10.0, got %q", got)
	}
}

func TestNewestVersionHandlesAlreadyVPrefixedCatalogueVersions(t *testing.T) {
	// The catalogue's version strings are not guaranteed bare. A duplicate
	// "v"+v.Version helper double-prefixes "v3.10.0" into "vv3.10.0", which
	// semver.Compare treats as invalid; invalid-vs-invalid compares equal, so
	// newest never advances past the first element - degrading straight back
	// into the Versions[0] bug this function exists to fix. Restoring that
	// buggy form must make this test go red.
	got := newestVersion([]models.ModVersion{
		{Version: "v3.2.1"},
		{Version: "v3.10.0"},
		{Version: "v3.3.0"},
	})

	if got != "v3.10.0" {
		t.Fatalf("expected v3.10.0, got %q", got)
	}
}

func TestNewestVersionIsEmptyForNoVersions(t *testing.T) {
	if got := newestVersion(nil); got != "" {
		t.Fatalf("expected no version, got %q", got)
	}
}
