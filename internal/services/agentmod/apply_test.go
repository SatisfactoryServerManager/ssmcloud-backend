package agentmod

import (
	"errors"
	"testing"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeQueue is a taskQueue that records what executePlan did, in order, and can
// simulate the dispatcher claiming the pending start mid-plan.
type fakeQueue struct {
	running      bool
	pendingSync  string
	pendingStart string

	// startClaimedOnRepoint makes the FIRST SetGate of the pending start report a
	// no-op, exactly as Mongo does once the dispatcher has moved it to running. The
	// server it boots is then reflected by flipping running to true, which is what
	// the re-plan must observe.
	startClaimedOnRepoint bool

	enqueued []enqueueCall
	gates    []gateCall
	plans    int
}

type enqueueCall struct {
	action string
	opts   agenttask.EnqueueOpts
}

type gateCall struct {
	taskID string
	opts   agenttask.EnqueueOpts
}

func (f *fakeQueue) ServerIsRunning(bson.ObjectID) (bool, error) {
	f.plans++
	return f.running, nil
}

func (f *fakeQueue) ReplacePendingPayload(bson.ObjectID, string, interface{}) (string, error) {
	return f.pendingSync, nil
}

func (f *fakeQueue) PendingIDByAction(_ bson.ObjectID, action string) (string, error) {
	if action == ActionStart {
		return f.pendingStart, nil
	}
	return "", nil
}

func (f *fakeQueue) Enqueue(_, _ bson.ObjectID, action string, _ interface{}, _ string, _ v2.TaskTrigger, opts agenttask.EnqueueOpts) (string, error) {
	f.enqueued = append(f.enqueued, enqueueCall{action: action, opts: opts})
	return bson.NewObjectID().Hex(), nil
}

func (f *fakeQueue) SetGate(taskID string, opts agenttask.EnqueueOpts) (bool, error) {
	f.gates = append(f.gates, gateCall{taskID: taskID, opts: opts})

	if taskID == f.pendingStart && f.startClaimedOnRepoint {
		// It has been claimed: it is running, so SetGate matches nothing, the game
		// server boots, and the start is no longer pending.
		f.startClaimedOnRepoint = false
		f.running = true
		f.pendingStart = ""
		return false, nil
	}
	return true, nil
}

func (f *fakeQueue) syncEnqueue(t *testing.T) enqueueCall {
	t.Helper()
	for _, e := range f.enqueued {
		if e.action == ActionSyncMods {
			return e
		}
	}
	t.Fatal("expected a syncmods to have been enqueued")
	return enqueueCall{}
}

// THE race the whole rework exists to close, and the one planFor cannot see: the
// plan's inputs are read without a lock, so an in-flight sync completing before
// the re-point lands cascades the pending start's gate off, the dispatcher claims
// it oldest-first, and the game server boots. The re-point then silently no-ops
// against a task that is no longer pending. Insert the sync on that plan's
// (now stale) `running=false` and an UNGATED syncmods rewrites the Mods directory
// under a live game.
//
// Revert executePlan's `if !matched { return errStartClaimed }` and this goes red:
// the sync ships with gateNone.
func TestEnqueueSyncNeverInsertsAnUngatedSyncWhenTheStartWasClaimedMidPlan(t *testing.T) {
	q := &fakeQueue{
		running:               false, // the chain's stop already ran
		pendingStart:          "652f000000000000000000b2",
		startClaimedOnRepoint: true, // ... and it gets claimed before our re-point lands
	}

	ids, err := enqueueSyncWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, false, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err != nil {
		t.Fatalf("expected the re-plan to succeed, got %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected the re-plan to enqueue the sync")
	}
	sync := q.syncEnqueue(t)
	if sync.opts.DependsOn == nil && !sync.opts.RequiresServerStopped {
		t.Fatal("a syncmods was inserted UNGATED while the game server was running: it will rewrite the Mods directory under a live game")
	}
	if !sync.opts.RequiresServerStopped {
		t.Fatalf("expected the re-plan to see running=true and gate the sync on requiresServerStopped, got %+v", sync.opts)
	}
	if q.plans < 2 {
		t.Fatalf("expected the lost re-point to force a re-plan, got %d plan(s)", q.plans)
	}
}

// The re-point must be attempted BEFORE the sync exists, not after. Gating second
// leaves the start simultaneously ungated and older than the new sync, which the
// oldest-first claim runs backwards.
func TestExecutePlanGatesThePendingStartBeforeInsertingTheSync(t *testing.T) {
	const pendingStart = "652f000000000000000000b2"
	q := &fakeQueue{pendingStart: pendingStart}

	if _, err := enqueueSyncWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, false, v2.TaskTrigger{Type: v2.TaskTriggerUser}); err != nil {
		t.Fatal(err)
	}

	if len(q.gates) == 0 || q.gates[0].taskID != pendingStart {
		t.Fatalf("expected the pending start to be re-gated first, got %+v", q.gates)
	}
	if q.gates[0].opts.DependsOn == nil {
		t.Fatal("expected the start to be gated onto the sync's pre-assigned _id")
	}
	sync := q.syncEnqueue(t)
	if sync.opts.ID == nil || *sync.opts.ID != *q.gates[0].opts.DependsOn {
		t.Fatal("expected the sync to be inserted with the very _id the start was gated onto")
	}
}

// A start that keeps being claimed out from under the plan must fail loudly. It
// must never fall through to inserting the sync, gated or not.
func TestEnqueueSyncGivesUpRatherThanInsertingASyncItCannotOrder(t *testing.T) {
	q := &alwaysClaimedQueue{}

	_, err := enqueueSyncWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, false, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err == nil {
		t.Fatal("expected an error once the plan attempts were exhausted")
	}
	if errors.Is(err, errStartClaimed) {
		t.Fatal("errStartClaimed is internal and must not reach the caller")
	}
	for _, e := range q.enqueued {
		if e.action == ActionSyncMods {
			t.Fatal("a syncmods was enqueued after the plan gave up")
		}
	}
	if q.plans != maxPlanAttempts {
		t.Fatalf("expected the retry to be bounded at %d attempts, got %d", maxPlanAttempts, q.plans)
	}
}

// alwaysClaimedQueue loses the re-point on every attempt.
type alwaysClaimedQueue struct{ fakeQueue }

func (a *alwaysClaimedQueue) SetGate(taskID string, opts agenttask.EnqueueOpts) (bool, error) {
	a.gates = append(a.gates, gateCall{taskID: taskID, opts: opts})
	return false, nil
}

func (a *alwaysClaimedQueue) PendingIDByAction(_ bson.ObjectID, action string) (string, error) {
	if action == ActionStart {
		return "652f000000000000000000b2", nil
	}
	return "", nil
}

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
