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

// shouldEscalate decides, given a pending sync already exists, whether
// enqueueSync must build the stop -> sync -> start chain instead of leaving
// the pending task exactly as it was replaced. The user is escalating from a
// deferred sync to "apply now" only when the server is running AND applyNow
// was requested; every other combination leaves the replaced task's own
// gating (already correct for its case) untouched.
func shouldEscalate(running, applyNow bool) bool {
	return running && applyNow
}

// enqueueSync puts the lockfile on the queue.
//
//   - Server stopped: one ungated task.
//   - Server running, applyNow: stop -> sync -> start, chained on dependsOn. The
//     chain's own stop satisfies the precondition, so the sync is NOT gated on
//     requiresServerStopped — gating it against a Status.Running that has not yet
//     been observed as false would deadlock the chain against a stale status write.
//   - Server running, deferred: one task gated on requiresServerStopped, claimed
//     within a dispatch tick of the server next stopping.
func enqueueSync(agentID, accountID bson.ObjectID, lf v2.Lockfile, applyNow bool, trigger v2.TaskTrigger) ([]string, error) {
	running, err := serverIsRunning(agentID)
	if err != nil {
		return nil, err
	}

	// A pending syncmods carries a lockfile that this change has just invalidated,
	// so collapse a burst of UI clicks by overwriting it in place. This only ever
	// matches status=pending: a RUNNING sync's payload is already in the agent's
	// hands, so there is nothing to overwrite, and the running task must be left
	// to finish and converge via the desired-state reconcile the handler already
	// performs. The pending task's existing gating (requiresServerStopped, or a
	// dependsOn chain) is left exactly as it was.
	replaced, err := agenttask.ReplacePendingPayload(agentID, ActionSyncMods, lf)
	if err != nil {
		return nil, err
	}
	// If the user is escalating a deferred sync into "apply now", the replaced
	// task's existing gate (requiresServerStopped) is exactly what must be
	// undone, so it is NOT safe to return early here: fall through and build
	// the stop -> sync -> start chain, adopting the replaced task as its sync.
	if replaced != "" && !shouldEscalate(running, applyNow) {
		if err := regatePendingStart(agentID, replaced); err != nil {
			return nil, err
		}
		return []string{replaced}, nil
	}

	// syncmods deliberately has NO dedupe key. uniq_active_dedupe's partial filter
	// covers "active", which is present for both pending AND RUNNING tasks — so a
	// dedupe key here would let Enqueue ADOPT a task that is already running and
	// mid-download, silently discarding this lockfile instead of shipping it. A
	// second sync enqueued while one is running is fine: the queue serialises per
	// agent, and the handler reconciles desired state, so the two converge. A lost
	// lockfile is not fine. At-most-one-PENDING is enforced above by
	// ReplacePendingPayload instead of by the index. Do not add a dedupe key back.
	const syncDedupe = ""

	if !running {
		id, err := agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, syncDedupe, trigger, agenttask.EnqueueOpts{})
		if err != nil {
			return nil, err
		}
		// INVARIANT: a pending startsfserver must always trail the NEWEST sync.
		// This is the ungated, no-chain path, so nothing else re-points a leftover
		// start left pending from an earlier "apply now" whose sync has since
		// finished (see enqueueSync's doc comment for the full scenario).
		if err := regatePendingStart(agentID, id); err != nil {
			return nil, err
		}
		return []string{id}, nil
	}

	if !applyNow {
		id, err := agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, syncDedupe, trigger,
			agenttask.EnqueueOpts{RequiresServerStopped: true})
		if err != nil {
			return nil, err
		}
		if err := regatePendingStart(agentID, id); err != nil {
			return nil, err
		}
		return []string{id}, nil
	}

	// Unlike the sync, the chain's stop/start DO want dedupe-adoption: a second
	// "apply now" click should re-point the sync at the SAME stop/start rather than
	// littering the task list with orphaned duplicates that still run harmlessly
	// but confuse the user.
	stopDedupe := "syncchain-stop:" + agentID.Hex()
	startDedupe := "syncchain-start:" + agentID.Hex()

	stopID, err := agenttask.Enqueue(agentID, accountID, ActionStop, nil, stopDedupe, trigger, agenttask.EnqueueOpts{})
	if err != nil {
		return nil, err
	}
	stopOID, err := bson.ObjectIDFromHex(stopID)
	if err != nil {
		return nil, err
	}

	// If ReplacePendingPayload found a pending sync above, reuse ITS id as the
	// chain's sync instead of enqueueing a second one: the sync has no dedupe
	// key (see above), so Enqueue cannot adopt it, and enqueueing anyway would
	// leave the replaced task orphaned — still pending, still gated on
	// requiresServerStopped, and liable to run a second, redundant sync the
	// moment the server stops on its own. Re-gating the existing task onto the
	// new stop turns it into the chain's sync instead of a stray duplicate.
	var syncID string
	if replaced != "" {
		syncID = replaced
		if err := agenttask.RegateForChain(syncID, stopOID); err != nil {
			return nil, err
		}
	} else {
		syncID, err = agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, syncDedupe, trigger,
			agenttask.EnqueueOpts{DependsOn: &stopOID})
		if err != nil {
			return nil, err
		}
	}
	syncOID, err := bson.ObjectIDFromHex(syncID)
	if err != nil {
		return nil, err
	}

	startID, err := agenttask.Enqueue(agentID, accountID, ActionStart, nil, startDedupe, trigger,
		agenttask.EnqueueOpts{DependsOn: &syncOID})
	if err != nil {
		return nil, err
	}
	// Enqueue's dedupe-adoption path ignores opts: if startDedupe adopted an
	// existing active start whose dependsOn still points at a stale sync, the
	// EnqueueOpts.DependsOn above never took effect. RegateForChain is
	// idempotent, so calling it unconditionally is free on the non-adopted
	// path and closes the gap on the adopted one.
	if err := agenttask.RegateForChain(startID, syncOID); err != nil {
		return nil, err
	}

	return []string{stopID, syncID, startID}, nil
}

// regatePendingStart re-points the agent's pending startsfserver, if any, at
// syncID.
//
// INVARIANT: a pending startsfserver must always trail the NEWEST sync. Any
// time enqueueSync settles on a sync's id — new, replaced-in-place, or part of
// a chain — a start left pending from an earlier "apply now" must be dragged
// forward onto it, or the dispatcher's FIFO claim can let that stale start run
// BEFORE this sync, booting the game server and then rewriting Mods underneath
// it. RegateForChain is idempotent and a no-op when there is no pending start.
func regatePendingStart(agentID bson.ObjectID, syncID string) error {
	start, err := agenttask.FindPendingByAction(agentID, ActionStart)
	if err != nil {
		return err
	}
	if start == nil {
		return nil
	}

	syncOID, err := bson.ObjectIDFromHex(syncID)
	if err != nil {
		return err
	}
	return agenttask.RegateForChain(start.ID.Hex(), syncOID)
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
