package agentmod

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
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
	ids := make(map[string]bson.ObjectID, len(lf.Mods))

	ModsModel, err := repositories.GetMongoClient().GetModel("Mod")
	if err != nil {
		return nil, err
	}

	for _, m := range lf.Mods {
		dbMod := &models.ModSchema{}
		if err := ModsModel.FindOne(dbMod, bson.M{"modReference": m.ModReference}); err != nil {
			return nil, fmt.Errorf("mod %s is not in the catalogue: %w", m.ModReference, err)
		}
		ids[m.ModReference] = dbMod.ID
	}

	return ids, nil
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

	// A pending syncmods carries a lockfile that this change has just invalidated.
	// Enqueue would adopt it on the dedupe key and ship the stale payload, so
	// overwrite it in place first, and only enqueue if that matched nothing —
	// the pending task's existing gating (requiresServerStopped, or a dependsOn
	// chain) is left exactly as it was.
	replaced, err := replacePendingSync(agentID, lf)
	if err != nil {
		return nil, err
	}
	if replaced != "" {
		return []string{replaced}, nil
	}

	dedupe := "syncmods:" + agentID.Hex()

	if !running {
		id, err := agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, dedupe, trigger, agenttask.EnqueueOpts{})
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	}

	if !applyNow {
		id, err := agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, dedupe, trigger,
			agenttask.EnqueueOpts{RequiresServerStopped: true})
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	}

	stopID, err := agenttask.Enqueue(agentID, accountID, ActionStop, nil, "", trigger, agenttask.EnqueueOpts{})
	if err != nil {
		return nil, err
	}
	stopOID, err := bson.ObjectIDFromHex(stopID)
	if err != nil {
		return nil, err
	}

	syncID, err := agenttask.Enqueue(agentID, accountID, ActionSyncMods, lf, dedupe, trigger,
		agenttask.EnqueueOpts{DependsOn: &stopOID})
	if err != nil {
		return nil, err
	}
	syncOID, err := bson.ObjectIDFromHex(syncID)
	if err != nil {
		return nil, err
	}

	startID, err := agenttask.Enqueue(agentID, accountID, ActionStart, nil, "", trigger,
		agenttask.EnqueueOpts{DependsOn: &syncOID})
	if err != nil {
		return nil, err
	}

	return []string{stopID, syncID, startID}, nil
}

// replacePendingSync overwrites the payload of an already-pending sync, returning
// its id, or "" if there was none.
func replacePendingSync(agentID bson.ObjectID, lf v2.Lockfile) (string, error) {
	payload, err := json.Marshal(lf)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	task := &v2.AgentTaskSchema{}
	err = repositories.GetMongoClient().
		GetCollection("agenttasks").
		FindOneAndUpdate(ctx,
			bson.M{"agentId": agentID, "action": ActionSyncMods, "status": v2.TaskStatusPending},
			bson.M{"$set": bson.M{"data": string(payload), "updatedAt": time.Now()}}).
		Decode(task)
	if err != nil {
		return "", nil // no pending sync; not an error
	}

	logger.GetDebugLogger().Printf("replaced the lockfile on pending sync %s", task.ID.Hex())
	return task.ID.Hex(), nil
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
