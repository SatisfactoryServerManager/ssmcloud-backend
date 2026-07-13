package agenttask

import (
	"testing"

	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

func TestDecideCascadeReleasesEveryChildOnCompletedParent(t *testing.T) {
	if got := decideCascade(v2.TaskStatusCompleted, "syncmods"); got != cascadeRelease {
		t.Fatalf("expected release on completed parent, got %v", got)
	}
	if got := decideCascade(v2.TaskStatusCompleted, recoveryExemptAction); got != cascadeRelease {
		t.Fatalf("expected release on completed parent, got %v", got)
	}
}

func TestDecideCascadeCancelsOrdinaryChildrenOnDeadOrCancelledParent(t *testing.T) {
	if got := decideCascade(v2.TaskStatusDead, "syncmods"); got != cascadeCancel {
		t.Fatalf("expected cancel for a non-exempt child of a dead parent, got %v", got)
	}
	if got := decideCascade(v2.TaskStatusCancelled, "syncmods"); got != cascadeCancel {
		t.Fatalf("expected cancel for a non-exempt child of a cancelled parent, got %v", got)
	}
}

// This is the invariant the whole cascade design hole revolves around: a
// startsfserver child must survive its parent dying or being cancelled, or the
// user's game server can be left stopped with nothing left to bring it back up.
func TestDecideCascadeReleasesStartServerOnDeadOrCancelledParent(t *testing.T) {
	if got := decideCascade(v2.TaskStatusDead, recoveryExemptAction); got != cascadeRelease {
		t.Fatalf("expected startsfserver to be released (not cancelled) when its parent died, got %v", got)
	}
	if got := decideCascade(v2.TaskStatusCancelled, recoveryExemptAction); got != cascadeRelease {
		t.Fatalf("expected startsfserver to be released (not cancelled) when its parent was cancelled, got %v", got)
	}
}
