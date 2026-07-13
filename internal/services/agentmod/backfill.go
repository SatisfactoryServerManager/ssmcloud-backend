package agentmod

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// legacySelectedMod is one entry of the embedded modConfig.selectedMods array
// this migrates out of. Read-only: nothing here ever writes back to it except
// the final $unset of the whole array.
type legacySelectedMod struct {
	ModID            bson.ObjectID `bson:"mod"`
	DesiredVersion   string        `bson:"desiredVersion"`
	InstalledVersion string        `bson:"installedVersion"`
	Installed        bool          `bson:"installed"`
	Config           string        `bson:"config"`
}

type legacyAgent struct {
	ID        bson.ObjectID `bson:"_id"`
	ModConfig struct {
		SelectedMods []legacySelectedMod `bson:"selectedMods"`
	} `bson:"modConfig"`
}

// backfillDoc builds the agentmods document for one legacy selection. Pure, so
// the migration's actual shape is testable without a database.
//
// direct is always true: the old array cannot distinguish a mod the user chose
// from a dependency the old code never tracked, and treating them all as direct
// preserves exactly the behaviour the user has today. The next resolve
// re-derives the real dependency set.
func backfillDoc(agentID, accountID, modID bson.ObjectID, modReference string, sm legacySelectedMod, now time.Time) v2.AgentModSchema {
	doc := v2.NewAgentModDoc(agentID, accountID, modID, modReference, sm.DesiredVersion, true)
	doc.InstalledVersion = sm.InstalledVersion
	doc.Installed = sm.Installed
	doc.Config = sm.Config
	doc.CreatedAt = now
	doc.UpdatedAt = now
	return doc
}

// resolvedMod pairs one legacy selection with its catalogue lookup, so
// backfillWrites can be pure: the I/O (modReferenceFor) happens in Backfill,
// the decision of what to write happens here, and only the decision needs a test.
type resolvedMod struct {
	sm  legacySelectedMod
	ref string
	err error
}

// backfillWrites turns one agent's resolved selections into the upserts to run
// against agentmods, and reports whether ANY mod failed to resolve.
//
// A single failure must NOT drop the mods that did resolve into a partial,
// silently-truncated migration, and it must NOT be papered over as success:
// the caller uses failed=true as the signal to leave modConfig in place so the
// agent is retried on the next boot instead of being trimmed to whatever
// happened to resolve on this pass.
func backfillWrites(agentID, accountID bson.ObjectID, resolved []resolvedMod, now time.Time) (writes []mongo.WriteModel, failed bool) {
	writes = make([]mongo.WriteModel, 0, len(resolved))

	for _, r := range resolved {
		if r.err != nil {
			failed = true
			continue
		}

		doc := backfillDoc(agentID, accountID, r.sm.ModID, r.ref, r.sm, now)

		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"agentId": agentID, "modReference": r.ref}).
			SetUpdate(bson.M{"$setOnInsert": doc}).
			SetUpsert(true))
	}

	return writes, failed
}

// Backfill migrates the old embedded modConfig.selectedMods array into agentmods.
//
// It is idempotent: an agent whose array has already been unset does not match
// the query below, so this is safe to run on every boot until it is deleted.
//
// A mod that is permanently absent from the catalogue (pruned, or a lookup
// that keeps erroring) will block that agent's migration on EVERY boot, and
// will say so in the error log every time. That is deliberate: a loud,
// recoverable stall is preferable to a silent, permanent loss of the agent's
// mod selection, and only an operator can tell the two failure modes apart.
func Backfill() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	agents := repositories.GetMongoClient().GetCollection("agents")

	// The 0-index existence check excludes agents whose array is present but
	// empty (every newly created agent, per v2.NewAgent) so the job actually
	// quiesces instead of doing an empty write/unset on them forever.
	cur, err := agents.Find(ctx, bson.M{"modConfig.selectedMods.0": bson.M{"$exists": true}})
	if err != nil {
		return err
	}

	legacy := make([]legacyAgent, 0)
	if err := cur.All(ctx, &legacy); err != nil {
		return err
	}

	if len(legacy) == 0 {
		return nil
	}

	migrated := 0

	for _, a := range legacy {
		accountID, err := agent.GetAccountIDForAgent(a.ID)
		if err != nil {
			logger.GetErrorLogger().Printf("backfill: no account for agent %s: %s", a.ID.Hex(), err.Error())
			continue
		}

		now := time.Now()
		resolved := make([]resolvedMod, 0, len(a.ModConfig.SelectedMods))

		for _, sm := range a.ModConfig.SelectedMods {
			ref, err := modReferenceFor(ctx, sm.ModID)
			if err != nil {
				logger.GetErrorLogger().Printf("backfill: agent %s mod %s not in catalogue: %s", a.ID.Hex(), sm.ModID.Hex(), err.Error())
			}
			resolved = append(resolved, resolvedMod{sm: sm, ref: ref, err: err})
		}

		writes, failed := backfillWrites(a.ID, accountID, resolved, now)

		if len(writes) > 0 {
			if _, err := collection().BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false)); err != nil {
				logger.GetErrorLogger().Printf("backfill: agent %s: %s", a.ID.Hex(), err.Error())
				// The array MUST stay: a partial/failed write plus an unset would
				// lose whatever did not make it into agentmods.
				continue
			}
		}

		if failed {
			// At least one mod failed to resolve: leave modConfig in place so this
			// agent is retried (idempotently — the writes above already landed) on
			// the next boot instead of silently losing the unresolved mod forever.
			continue
		}

		// $unset only after every mod resolved AND the upserts succeeded: an agent
		// whose array is unset is not matched by this job's own query, so any
		// failure above must leave the array in place or this migration silently
		// loses the agent's mods.
		if _, err := agents.UpdateOne(ctx,
			bson.M{"_id": a.ID},
			bson.M{"$unset": bson.M{"modConfig": ""}}); err != nil {
			logger.GetErrorLogger().Printf("backfill: cannot unset modConfig on agent %s: %s", a.ID.Hex(), err.Error())
			continue
		}

		migrated++
	}

	logger.GetDebugLogger().Printf("backfilled mods for %d agents", migrated)
	return nil
}

func modReferenceFor(ctx context.Context, modID bson.ObjectID) (string, error) {
	var m struct {
		ModReference string `bson:"modReference"`
	}
	err := repositories.GetMongoClient().GetCollection("mods").
		FindOne(ctx, bson.M{"_id": modID}).Decode(&m)
	return m.ModReference, err
}

// Init prepares the collection at boot: indexes first, then the migration. Call
// this once during boot, next to agenttask.EnsureIndexes() and before
// agenttask.StartDispatcher() - the migration must finish before any task can be
// dispatched against a lockfile that assumes agentmods already holds the
// agent's selection.
func Init() error {
	if err := EnsureIndexes(); err != nil {
		return err
	}
	if err := Backfill(); err != nil {
		return err
	}
	return nil
}
