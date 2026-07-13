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

func TestNewestVersionIsEmptyForNoVersions(t *testing.T) {
	if got := newestVersion(nil); got != "" {
		t.Fatalf("expected no version, got %q", got)
	}
}
