package agentmod

import (
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
