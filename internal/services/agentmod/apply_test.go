package agentmod

import (
	"errors"
	"testing"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeQueue is a taskQueue that records what executePlan did, in order, and can
// simulate both races the plan has to survive: the dispatcher claiming the pending
// start mid-plan, and Enqueue's dedupe key ADOPTING a start that is already running.
type fakeQueue struct {
	running      bool
	pendingSync  string
	pendingStart string

	// runningStartID is a startsfserver the dispatcher has already claimed. The agent
	// has NOT yet reported running:true — the game is still booting — which is exactly
	// why agents.status.running cannot be the plan's only notion of "running".
	//
	// It is adoptable (uniq_active_dedupe covers running) and it is NOT re-gatable
	// (SetGate matches pending only), which together are the trap of finding 2.
	runningStartID string

	// startClaimedOnRepoint makes the FIRST SetGate of the pending start report a
	// no-op, exactly as Mongo does once the dispatcher has moved it to running. The
	// server it boots is then reflected by flipping running to true, which is what
	// the re-plan must observe.
	startClaimedOnRepoint bool

	// startAppearsAfterSyncInsert simulates a startsfserver enqueued by someone
	// else AFTER PendingIDByAction reported "" (so RepointStartID was empty and
	// the read-time re-point never ran) but BEFORE HasActiveAction is checked
	// again once the sync exists. HasActiveAction flips to true the moment the
	// sync has been inserted.
	startAppearsAfterSyncInsert bool
	syncInserted                bool

	enqueued []enqueueCall
	gates    []gateCall
	plans    int
}

type enqueueCall struct {
	action    string
	dedupeKey string
	opts      agenttask.EnqueueOpts
}

type gateCall struct {
	taskID string
	opts   agenttask.EnqueueOpts
}

func (f *fakeQueue) ServerIsRunning(bson.ObjectID) (bool, error) {
	f.plans++
	return f.running, nil
}

// An active start is a pending one or a running one. Mirrors agenttask's `active`
// field, which is present for exactly those two statuses.
func (f *fakeQueue) HasActiveAction(_ bson.ObjectID, action string) (bool, error) {
	if action != ActionStart {
		return false, nil
	}
	if f.startAppearsAfterSyncInsert && f.syncInserted {
		return true, nil
	}
	return f.pendingStart != "" || f.runningStartID != "", nil
}

func (f *fakeQueue) ReplacePendingPayload(bson.ObjectID, string, interface{}) (string, error) {
	return f.pendingSync, nil
}

func (f *fakeQueue) PendingIDByAction(_ bson.ObjectID, action string) (string, error) {
	switch action {
	case ActionStart:
		return f.pendingStart, nil
	case ActionSyncMods:
		// The real one is generic over the action; a fake that answered only for
		// startsfserver would let a caller that asks about a pending SYNC (the
		// escalation guard in applyPendingNowWith) pass its test against a lie.
		return f.pendingSync, nil
	}
	return "", nil
}

func (f *fakeQueue) Enqueue(agentID, _ bson.ObjectID, action string, _ interface{}, dedupeKey string, _ v2.TaskTrigger, opts agenttask.EnqueueOpts) (string, error) {
	f.enqueued = append(f.enqueued, enqueueCall{action: action, dedupeKey: dedupeKey, opts: opts})

	if action == ActionSyncMods {
		f.syncInserted = true
	}

	// The chain's agent-wide start key collides with the still-RUNNING start, so
	// Enqueue adopts it and silently drops opts — which is how a new chain ends up
	// with no start of its own at all.
	if action == ActionStart && f.runningStartID != "" && dedupeKey == "syncchain-start:"+agentID.Hex() {
		return f.runningStartID, nil
	}
	return bson.NewObjectID().Hex(), nil
}

func (f *fakeQueue) SetGate(taskID string, opts agenttask.EnqueueOpts) (bool, error) {
	f.gates = append(f.gates, gateCall{taskID: taskID, opts: opts})

	// Already running: SetGate matches status=pending, so it lands on nothing.
	if taskID != "" && taskID == f.runningStartID {
		return false, nil
	}

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

func (f *fakeQueue) enqueuesOf(action string) []enqueueCall {
	out := make([]enqueueCall, 0, len(f.enqueued))
	for _, e := range f.enqueued {
		if e.action == action {
			out = append(out, e)
		}
	}
	return out
}

// FINDING 1. The agent-reported status is blind to task state: it is still false
// for the whole time a startsfserver is mid-boot. A plan that reads only that field
// therefore decides "stopped" while the game is coming up and gates NOTHING —
// uniq_running_per_agent holds the sync back only until the start finishes, by which
// point the game is live and the ungated sync rewrites Mods underneath it. The
// staleness is not one-directional: a stale false UNDER-gates.
//
// Revert planRunning's HasActiveAction OR and this goes red: the sync ships ungated.
func TestEnqueueSyncGatesTheSyncWhileAStartIsStillBooting(t *testing.T) {
	q := &fakeQueue{
		running:        false, // the agent has not reported the boot yet ...
		runningStartID: "652f000000000000000000c3",
	} // ... but its startsfserver is RUNNING right now

	if _, err := enqueueSyncWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, false, v2.TaskTrigger{Type: v2.TaskTriggerUser}); err != nil {
		t.Fatal(err)
	}

	sync := q.syncEnqueue(t)
	if sync.opts.DependsOn == nil && !sync.opts.RequiresServerStopped {
		t.Fatal("a syncmods was inserted UNGATED while a startsfserver was mid-boot: it becomes claimable the moment the game is live and rewrites the Mods directory under it")
	}
}

// FINDING 2. uniq_active_dedupe is partial on `active`, which covers RUNNING, so the
// chain's agent-wide start key ADOPTS the previous chain's still-booting start.
// SetGate then correctly no-ops against it (it is not pending), and the new chain is
// stop -> sync -> NOTHING: the server goes down for the sync and never comes back.
// Invariant 3.
//
// Revert ensureTrailingStart's `if matched { return }` / fresh-insert fallback and
// this goes red: the chain's only start IS the running one.
func TestEnqueueSyncNeverAdoptsARunningStartAsItsOwnTail(t *testing.T) {
	const runningStart = "652f000000000000000000c3"

	agentID := bson.NewObjectID()
	q := &fakeQueue{running: false, runningStartID: runningStart}

	ids, err := enqueueSyncWith(q, agentID, bson.NewObjectID(), v2.Lockfile{}, true, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err != nil {
		t.Fatal(err)
	}

	starts := q.enqueuesOf(ActionStart)
	if len(starts) != 2 {
		t.Fatalf("expected the adopted running start to be rejected and a fresh one inserted, got %d start enqueue(s)", len(starts))
	}
	if starts[1].dedupeKey == starts[0].dedupeKey {
		t.Fatal("the fresh start reused the agent-wide dedupe key, so it adopts the very running start it exists to replace")
	}
	if starts[1].opts.DependsOn == nil {
		t.Fatal("the chain's own start must trail its sync")
	}

	// ids is [sync, start, stop]: the tail must not be the start that was already
	// booting, or nothing brings the server back up after the sync.
	if len(ids) != 3 {
		t.Fatalf("expected sync + start + stop, got %v", ids)
	}
	if ids[1] == runningStart {
		t.Fatal("the chain adopted the RUNNING start as its tail: it cannot be gated behind the sync, so the server goes down for the sync and never comes back up")
	}
	if len(q.enqueuesOf(ActionStop)) != 1 {
		t.Fatal("expected the chain to take the server down before the sync")
	}
}

// FINDING 3. An aborted attempt must leave NOTHING behind. Parents go in last, so
// the only step that can abort — the re-point of a pre-existing pending start, which
// is an UPDATE — runs before any insert. A stopsfserver written before it would
// survive the abort, run on its own, and take the user's game server down with no
// sync and no start behind it.
//
// Revert executePlan's parents-last ordering (write the stop first) and this goes
// red: three exhausted attempts leave three stopsfservers pending.
func TestEnqueueSyncLeavesNoTasksBehindWhenTheAttemptIsAborted(t *testing.T) {
	q := &alwaysClaimedQueue{fakeQueue{running: true}}

	_, err := enqueueSyncWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, true, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err == nil {
		t.Fatal("expected an error once the plan attempts were exhausted")
	}

	if len(q.enqueued) != 0 {
		t.Fatalf("an aborted plan wrote tasks it can no longer order: %+v", q.enqueued)
	}
	for _, e := range q.enqueued {
		if e.action == ActionStop {
			t.Fatal("an orphaned stopsfserver survived the abort: it will run on its own and leave the user's server down with nothing to sync or restart it")
		}
	}
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

// THE remaining hole: HasActiveAction at plan-read time is a READ, not a fence. It
// closes the case where a start already exists when the plan's inputs are read,
// but not one that appears AFTER PendingIDByAction returned "" and the plan lands
// with gateNone -- there is no pending start to re-point onto, so
// errStartClaimed's abort never fires and nothing re-plans.
//
// Revert executePlan's post-insert re-check and this goes red: the sync ships
// with gateNone while a startsfserver is active.
func TestEnqueueSyncReGatesTheSyncWhenAStartAppearsAfterItIsInserted(t *testing.T) {
	q := &fakeQueue{
		running:                     false, // stopped at read time: no pending or running start either
		startAppearsAfterSyncInsert: true,   // ... but one is claimed the instant the sync exists
	}

	ids, err := enqueueSyncWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, false, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err != nil {
		t.Fatalf("expected the plan to succeed, got %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected the sync to have been enqueued")
	}

	sync := q.syncEnqueue(t)
	if sync.opts.DependsOn != nil || sync.opts.RequiresServerStopped {
		t.Fatalf("expected the sync to be inserted with gateNone before the post-insert check, got %+v", sync.opts)
	}

	found := false
	for _, g := range q.gates {
		if g.taskID == sync.opts.ID.Hex() && g.opts.RequiresServerStopped {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the post-insert check to re-gate the sync onto requiresServerStopped once the start appeared, got gates=%+v", q.gates)
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

// ApplyPendingNow's ENTIRE reason for existing. The selection was already
// persisted when the change was deferred, so the lockfile it resolves is the one
// the agent's rows already describe: Apply's diff is EMPTY and it correctly
// returns []string{} without building anything. But the deferred sync is real and
// is sitting on requiresServerStopped, so "Apply now" must still build the
// stop -> sync -> start chain around it. Routing this through Apply is a silent
// no-op — the exact bug the pending banner's button shipped with.
//
// Make applyPendingNowWith short-circuit the way Apply does (return []string{},
// nil before enqueueSyncWith) and this goes red on the assertions below.
func TestApplyPendingNowBuildsAChainForAnAlreadyPersistedChange(t *testing.T) {
	const pendingSync = "652f000000000000000000a1"

	// The server is running and the deferred sync is pending: exactly the state the
	// "Mods pending" banner renders over.
	q := &fakeQueue{running: true, pendingSync: pendingSync}

	ids, err := applyPendingNowWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err != nil {
		t.Fatal(err)
	}

	if len(ids) == 0 {
		t.Fatal("ApplyPendingNow enqueued nothing: the deferred sync stays gated on requiresServerStopped and the user's 'Apply now' is a silent no-op")
	}
	if len(q.enqueuesOf(ActionStop)) != 1 {
		t.Fatal("expected the escalation to stop the server: nothing else releases the deferred sync's gate")
	}
	if len(q.enqueuesOf(ActionStart)) != 1 {
		t.Fatal("expected a trailing startsfserver: an apply-now that stops the server and never restarts it leaves the game DOWN")
	}

	// The deferred sync is REUSED, not duplicated, and it must come off
	// requiresServerStopped and onto the chain's stop — otherwise the stop takes the
	// game down and the sync, still waiting on a status that already read stopped
	// before the chain began, may never be claimed.
	if len(q.enqueuesOf(ActionSyncMods)) != 0 {
		t.Fatal("expected the pending deferred sync to be reused, not a second one inserted")
	}
	regated := false
	for _, g := range q.gates {
		if g.taskID == pendingSync {
			if g.opts.RequiresServerStopped {
				t.Fatal("the reused sync kept requiresServerStopped while being gated onto the chain's stop")
			}
			if g.opts.DependsOn != nil {
				regated = true
			}
		}
	}
	if !regated {
		t.Fatalf("expected the pending sync to be re-gated onto the chain's stopsfserver, got gates=%+v", q.gates)
	}
}

// The stale-banner case. The deferred sync has already run (or was cancelled), but
// the tasks poll has not refreshed the page, so the "Mods pending" banner and its
// Apply-now button are still on screen. Clicking it must do NOTHING.
//
// Without the pending-sync guard, planFor gets an empty pendingSyncID plus
// (running && applyNow) and builds a WHOLE NEW chain: a healthy running server is
// stopped, synced to the state it is already in, and restarted - every player
// kicked, with no preview and no confirmation in the way. The diff check that
// normally catches a no-op change is deliberately skipped on this path, so this
// guard is the only thing in front of it.
func TestApplyPendingNowDoesNothingWithNoPendingSync(t *testing.T) {
	q := &fakeQueue{running: true} // running server, NO pending sync

	ids, err := applyPendingNowWith(q, bson.NewObjectID(), bson.NewObjectID(), v2.Lockfile{}, v2.TaskTrigger{Type: v2.TaskTriggerUser})
	if err != nil {
		t.Fatal(err)
	}

	if len(ids) != 0 {
		t.Fatalf("expected no tasks, got %v", ids)
	}
	if len(q.enqueuesOf(ActionStop)) != 0 {
		t.Fatal("a stale Apply-now click stopped a healthy running server: there was no deferred sync to escalate")
	}
	if len(q.enqueuesOf(ActionStart)) != 0 || len(q.enqueuesOf(ActionSyncMods)) != 0 {
		t.Fatal("a stale Apply-now click built a spurious chain")
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
