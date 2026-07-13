package agentmod

import (
	"context"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	ActionSyncMods = "syncmods"
	ActionStop     = "stopsfserver"
	ActionStart    = "startsfserver"
)

const (
	OpAdd        = "add"
	OpRemove     = "remove"
	OpSetVersion = "setVersion"
	OpUpdateAll  = "updateAll"
)

// ModChange is one user action on the mod selection.
type ModChange struct {
	Op           string
	ModReference string
	Version      string
}

// nextSelection turns a user action into the direct selection to resolve.
//
// The map is modReference -> pinned version, where "" means "latest compatible"
// and lets the resolver choose. Only direct mods appear: a dependency is the
// resolver's business, and pinning one here would stop it ever being dropped.
func nextSelection(current []v2.AgentModSchema, ch ModChange, latest map[string]string) map[string]string {
	sel := make(map[string]string)

	for _, m := range current {
		if !m.Direct {
			continue
		}
		sel[m.ModReference] = m.DesiredVersion
	}

	switch ch.Op {
	case OpAdd:
		sel[ch.ModReference] = ""
	case OpRemove:
		delete(sel, ch.ModReference)
	case OpSetVersion:
		sel[ch.ModReference] = ch.Version
	case OpUpdateAll:
		for ref := range sel {
			if l, ok := latest[ref]; ok && l != "" {
				sel[ref] = l
			}
		}
	}

	return sel
}

// latestVersions reads the catalogue's newest version per mod out of the agent's
// own rows, which the catalogue job keeps current.
func latestVersions(current []v2.AgentModSchema) map[string]string {
	out := make(map[string]string, len(current))
	for _, m := range current {
		if m.LatestVersion != "" {
			out[m.ModReference] = m.LatestVersion
		}
	}
	return out
}

// Preview resolves a hypothetical change and returns what it would do. Nothing is
// written. A resolution failure is returned as an error, which is what puts
// "cannot add X: requires SF >= 1.1" in the dialog instead of in a dead task.
func Preview(agentID bson.ObjectID, ch ModChange) (Change, error) {
	current, err := ListForAgent(agentID)
	if err != nil {
		return Change{}, err
	}

	next, err := ResolveSelection(agentID, nextSelection(current, ch, latestVersions(current)))
	if err != nil {
		return Change{}, err
	}

	return Diff(current, next), nil
}

// Apply resolves the change, persists it, and enqueues the work.
//
// applyNow is only consulted when the server is running: a stopped server needs
// no chain and no gate.
func Apply(agentID, accountID bson.ObjectID, ch ModChange, applyNow bool, trigger v2.TaskTrigger) ([]string, error) {
	current, err := ListForAgent(agentID)
	if err != nil {
		return nil, err
	}

	lf, err := ResolveSelection(agentID, nextSelection(current, ch, latestVersions(current)))
	if err != nil {
		return nil, err
	}

	if Diff(current, lf).IsEmpty() {
		return []string{}, nil
	}

	if err := persist(agentID, accountID, lf); err != nil {
		return nil, err
	}

	return enqueueSync(agentID, accountID, lf, applyNow, trigger)
}

// ApplyConfigOnly persists an edit to a mod's .cfg text and enqueues a sync.
//
// A config-text edit moves no versions, so Diff on the resulting lockfile is
// empty and Apply's diff-empty short circuit would silently drop it — but the
// text only reaches the agent's disk inside ModLock.Config during a sync, so
// skipping the enqueue here would leave the running server on the old config
// indefinitely. The diff check is therefore skipped deliberately, not an
// oversight: for a config-only change, "the lockfile didn't change" says
// nothing about whether the agent needs to re-sync.
func ApplyConfigOnly(agentID, accountID bson.ObjectID, modReference, config string, applyNow bool, trigger v2.TaskTrigger) ([]string, error) {
	if err := SetConfig(agentID, modReference, config); err != nil {
		return nil, err
	}

	lf, err := Resolve(agentID)
	if err != nil {
		return nil, err
	}

	return enqueueSync(agentID, accountID, lf, applyNow, trigger)
}

// persist writes the resolved lockfile back as the agent's selection.
func persist(agentID, accountID bson.ObjectID, lf v2.Lockfile) error {
	modIDs, err := catalogueIDs(lf)
	if err != nil {
		return err
	}

	if err := UpsertMany(agentID, accountID, lf.Mods, modIDs); err != nil {
		return err
	}

	// keep must never be nil: DeleteAbsent refuses a nil keep list, and an
	// explicit empty slice is what lets the user remove their last mod.
	keep := make([]string, 0, len(lf.Mods))
	for _, m := range lf.Mods {
		keep = append(keep, m.ModReference)
	}

	return DeleteAbsent(agentID, keep)
}

func catalogueIDs(lf v2.Lockfile) (map[string]bson.ObjectID, error) {
	refs := make([]string, 0, len(lf.Mods))
	for _, m := range lf.Mods {
		refs = append(refs, m.ModReference)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cur, err := repositories.GetMongoClient().GetCollection("mods").
		Find(ctx, bson.M{"modReference": bson.M{"$in": refs}})
	if err != nil {
		return nil, err
	}

	var dbMods []models.ModSchema
	if err := cur.All(ctx, &dbMods); err != nil {
		return nil, err
	}

	ids := make(map[string]bson.ObjectID, len(dbMods))
	for _, dbMod := range dbMods {
		ids[dbMod.ModReference] = dbMod.ID
	}

	// The $in query silently drops any modReference not in the catalogue, so the
	// missing-mod error has to be reconstructed by diffing the result against the
	// request rather than surfacing from the query itself.
	for _, ref := range refs {
		if _, ok := ids[ref]; !ok {
			return nil, fmt.Errorf("mod %s is not in the catalogue", ref)
		}
	}

	return ids, nil
}

// syncGate is the single gate a syncmods carries. The three values are mutually
// exclusive by construction, which is what makes the escalation deadlock
// unrepresentable: a sync gated behind the chain's own stop CANNOT also be gated
// on requiresServerStopped, because that condition is evaluated against a
// Status.Running the stop has not yet been observed to have cleared.
type syncGate int

const (
	gateNone          syncGate = iota // claimable now; the server is already stopped
	gateAfterStop                     // dependsOn the chain's stopsfserver
	gateServerStopped                 // requiresServerStopped; claimed whenever the server next stops
)

// syncPlan is the whole decision. The executor does nothing this does not say.
type syncPlan struct {
	NeedStop       bool     // enqueue (or adopt) the chain's stopsfserver
	ReuseSyncID    string   // "" => insert a fresh syncmods; else re-gate this pending one
	Gate           syncGate // the gate the sync ends up carrying, whether reused or fresh
	EnsureStart    bool     // enqueue (or adopt) a startsfserver trailing the sync
	RepointStartID string   // "" => none; else the pending start to drag onto the sync
}

// planFor is the whole of enqueueSync's correctness, as a pure function of the
// only four inputs that matter.
//
// INVARIANT 1: a syncmods must never be claimable while the game is running —
// hence a gate on every running path.
// INVARIANT 2: a pending startsfserver must always trail the NEWEST sync —
// hence RepointStartID is set whenever a start is pending, on EVERY path. A
// start left pending from an earlier chain is older than the sync we are about
// to add, and the dispatcher claims oldest-first, so leaving it ungated boots
// the game and then rewrites Mods underneath it.
//
// A pending sync is always REUSED, never duplicated: it has no dedupe key, so
// Enqueue cannot collapse a second one, and the orphan would re-run the sync the
// next time the server stopped.
//
// Re-pointing a leftover start from a CANCELLED chain (server deliberately left
// stopped) is still right: that start is already pending and already ungated, so
// it is going to boot the server regardless — refusing to re-point does not
// prevent the boot, it only lets it happen BEFORE the sync instead of after.
func planFor(running, applyNow bool, pendingSyncID, pendingStartID string) syncPlan {
	p := syncPlan{ReuseSyncID: pendingSyncID, RepointStartID: pendingStartID}

	switch {
	case running && applyNow:
		p.NeedStop = true
		p.Gate = gateAfterStop
		p.EnsureStart = true
	case running:
		p.Gate = gateServerStopped
	default:
		p.Gate = gateNone
	}

	return p
}

// syncmods deliberately has NO dedupe key. uniq_active_dedupe's partial filter
// covers "active", which is present for pending AND RUNNING tasks, so a key here
// would let Enqueue ADOPT a mid-download sync and silently discard this lockfile.
// At-most-one-PENDING is enforced by ReplacePendingPayload instead of by the
// index. Do not add a dedupe key back.
const syncDedupe = ""

// enqueueSync gathers the four facts planFor needs, then executes its plan.
func enqueueSync(agentID, accountID bson.ObjectID, lf v2.Lockfile, applyNow bool, trigger v2.TaskTrigger) ([]string, error) {
	running, err := serverIsRunning(agentID)
	if err != nil {
		return nil, err
	}

	// Collapses a burst of UI clicks. Matches status=pending only: a running sync's
	// payload is already in the agent's hands, and its handler reconciles desired
	// state, so the two converge.
	pendingSync, err := agenttask.ReplacePendingPayload(agentID, ActionSyncMods, lf)
	if err != nil {
		return nil, err
	}

	pendingStart := ""
	start, err := agenttask.FindPendingByAction(agentID, ActionStart)
	if err != nil {
		return nil, err
	}
	if start != nil {
		pendingStart = start.ID.Hex()
	}

	return executePlan(agentID, accountID, lf, trigger, planFor(running, applyNow, pendingSync, pendingStart))
}

// executePlan executes a syncPlan. It makes no decisions; the ordering below is
// the only thing it is responsible for.
func executePlan(agentID, accountID bson.ObjectID, lf v2.Lockfile, trigger v2.TaskTrigger, p syncPlan) ([]string, error) {
	ids := make([]string, 0, 3)

	// The chain's stop/start DO want dedupe-adoption: a second "apply now" should
	// re-point at the same pair rather than litter the task list with duplicates.
	var stopOID *bson.ObjectID
	if p.NeedStop {
		stopID, err := agenttask.Enqueue(agentID, accountID, ActionStop, nil,
			"syncchain-stop:"+agentID.Hex(), trigger, agenttask.EnqueueOpts{})
		if err != nil {
			return nil, err
		}
		oid, err := bson.ObjectIDFromHex(stopID)
		if err != nil {
			return nil, err
		}
		stopOID = &oid
		ids = append(ids, stopID)
	}

	gate := agenttask.EnqueueOpts{}
	switch p.Gate {
	case gateAfterStop:
		gate.DependsOn = stopOID
	case gateServerStopped:
		gate.RequiresServerStopped = true
	}

	// Pre-assign the sync's _id so the pending start can be gated onto it BEFORE the
	// sync row exists. Gating second would leave a window in which the start is both
	// ungated and OLDER than the new sync: an in-flight sync completing inside it
	// cascades the start's gate off, the oldest-first claim takes the start, and the
	// game boots into a Mods rewrite. Order, not timing, is what closes this.
	syncOID := bson.NewObjectID()
	if p.ReuseSyncID != "" {
		oid, err := bson.ObjectIDFromHex(p.ReuseSyncID)
		if err != nil {
			return nil, err
		}
		syncOID = oid
	}

	if p.RepointStartID != "" {
		if err := agenttask.SetGate(p.RepointStartID, agenttask.EnqueueOpts{DependsOn: &syncOID}); err != nil {
			return nil, err
		}
	}

	if p.ReuseSyncID != "" {
		if err := agenttask.SetGate(p.ReuseSyncID, gate); err != nil {
			return nil, err
		}
	} else {
		gate.ID = &syncOID
		if _, err := agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, syncDedupe, trigger, gate); err != nil {
			// The start is now gated behind an _id that will never exist. Release it;
			// the dead/cancelled cascade's recovery-exempt release covers the rest.
			if p.RepointStartID != "" {
				_ = agenttask.SetGate(p.RepointStartID, agenttask.EnqueueOpts{})
			}
			return nil, err
		}
	}
	ids = append(ids, syncOID.Hex())

	if p.EnsureStart {
		startID, err := agenttask.Enqueue(agentID, accountID, ActionStart, nil,
			"syncchain-start:"+agentID.Hex(), trigger, agenttask.EnqueueOpts{DependsOn: &syncOID})
		if err != nil {
			return nil, err
		}
		// Adoption ignores opts, so an adopted start may still carry a stale gate.
		// SetGate is idempotent; on the freshly-inserted path this is a free no-op.
		if err := agenttask.SetGate(startID, agenttask.EnqueueOpts{DependsOn: &syncOID}); err != nil {
			return nil, err
		}
		ids = append(ids, startID)
	}

	return ids, nil
}

func serverIsRunning(agentID bson.ObjectID) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc struct {
		Status struct {
			Running bool `bson:"running"`
		} `bson:"status"`
	}

	if err := repositories.GetMongoClient().GetCollection("agents").
		FindOne(ctx, bson.M{"_id": agentID}).Decode(&doc); err != nil {
		return false, err
	}
	return doc.Status.Running, nil
}
