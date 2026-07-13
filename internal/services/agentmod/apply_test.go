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

// planFor is the whole of enqueueSync's correctness. Every row below is a real
// scenario, and rows C, E and F are each a bug that shipped: revert the fix and
// that row goes red.
func TestPlanFor(t *testing.T) {
	const (
		pendingSync  = "652f000000000000000000a1"
		pendingStart = "652f000000000000000000b2"
	)

	cases := []struct {
		name           string
		running        bool
		applyNow       bool
		pendingSyncID  string
		pendingStartID string
		want           syncPlan
	}{
		{
			// A. The plain chain. The sync is gated on the chain's own stop and NOT on
			// requiresServerStopped: that condition reads a Status.Running the stop has
			// not yet been observed to have cleared, so gating on it would deadlock the
			// chain against a stale status write.
			name:    "A running, apply now, nothing pending -> stop -> sync -> start",
			running: true, applyNow: true,
			want: syncPlan{NeedStop: true, Gate: gateAfterStop, EnsureStart: true},
		},
		{
			// B. Deferred: no chain to build, so the sync must gate itself.
			name:    "B running, deferred, nothing pending -> one requiresServerStopped sync",
			running: true, applyNow: false,
			want: syncPlan{Gate: gateServerStopped},
		},
		{
			// C. ESCALATION (shipped bug #1). The deferred sync is REUSED, not
			// duplicated, and its requiresServerStopped is CLEARED as it moves onto the
			// stop. If that gate survived, the stop would run, the game would go down,
			// the sync would never be claimable, and the server would stay down forever.
			name:    "C escalation: pending deferred sync is reused, re-gated onto the stop, requiresServerStopped cleared",
			running: true, applyNow: true, pendingSyncID: pendingSync,
			want: syncPlan{NeedStop: true, ReuseSyncID: pendingSync, Gate: gateAfterStop, EnsureStart: true},
		},
		{
			name:    "D stopped, nothing pending -> one ungated sync",
			running: false, applyNow: false,
			want: syncPlan{Gate: gateNone},
		},
		{
			// E. FIFO bug (shipped bug #2). The chain's stop has already run, its sync
			// is RUNNING (so no pending sync to reuse), its start is still PENDING. A
			// fresh sync is newer than that start, and the dispatcher claims
			// oldest-first — without the re-point the start is claimed FIRST, the game
			// boots, and the new sync then rewrites Mods underneath it.
			name:    "E stopped mid-chain, sync running, start pending -> new sync, start re-pointed onto it",
			running: false, applyNow: true, pendingStartID: pendingStart,
			want: syncPlan{Gate: gateNone, RepointStartID: pendingStart},
		},
		{
			// F. Leftover start from a CANCELLED chain, server deliberately left
			// stopped. Re-pointing does not resurrect anything: that start is already
			// pending and already ungated, so it will boot the server either way.
			// Declining to re-point would not prevent the boot — it would only let the
			// boot happen BEFORE the sync, which is exactly invariant 2's failure.
			name:    "F stopped, leftover pending start from a cancelled chain -> still re-pointed, never left ahead of the sync",
			running: false, applyNow: false, pendingStartID: pendingStart,
			want: syncPlan{Gate: gateNone, RepointStartID: pendingStart},
		},
		{
			// Deferred escalation's mirror: reuse the pending sync but leave it on its
			// own gate, and never ensure a start the user did not ask for.
			name:    "running, deferred, pending sync -> reused, still requiresServerStopped, no start",
			running: true, applyNow: false, pendingSyncID: pendingSync, pendingStartID: pendingStart,
			want: syncPlan{ReuseSyncID: pendingSync, Gate: gateServerStopped, RepointStartID: pendingStart},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := planFor(c.running, c.applyNow, c.pendingSyncID, c.pendingStartID)
			if got != c.want {
				t.Fatalf("planFor(running=%v, applyNow=%v, sync=%q, start=%q)\n got: %+v\nwant: %+v",
					c.running, c.applyNow, c.pendingSyncID, c.pendingStartID, got, c.want)
			}
		})
	}
}

// A syncmods is claimable while the game runs only if it carries no gate. Every
// running-server plan must therefore carry one — invariant 1, stated where it can
// actually fail rather than in a comment.
func TestPlanForNeverLeavesASyncUngatedWhileTheServerRuns(t *testing.T) {
	for _, applyNow := range []bool{true, false} {
		for _, sync := range []string{"", "652f000000000000000000a1"} {
			p := planFor(true, applyNow, sync, "")
			if p.Gate == gateNone {
				t.Fatalf("planFor(running=true, applyNow=%v, sync=%q) left the sync ungated", applyNow, sync)
			}
			if p.Gate == gateAfterStop && !p.NeedStop {
				t.Fatalf("planFor(running=true, applyNow=%v, sync=%q) gated the sync on a stop it never enqueues", applyNow, sync)
			}
		}
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
