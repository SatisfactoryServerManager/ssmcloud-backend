package agentmod

import (
	"context"
	"errors"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const collectionName = "agentmods"

func collection() *mongo.Collection {
	return repositories.GetMongoClient().GetCollection(collectionName)
}

// EnsureIndexes creates the indexes the collection's correctness depends on.
// uniq_agent_mod is the upsert key and the guarantee that one agent cannot hold
// two rows for the same mod.
func EnsureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "agentId", Value: 1}, {Key: "modReference", Value: 1}},
			Options: options.Index().
				SetName("uniq_agent_mod").
				SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetName("by_agent"),
		},
		{
			Keys:    bson.D{{Key: "modId", Value: 1}},
			Options: options.Index().SetName("by_mod"),
		},
	}

	if _, err := collection().Indexes().CreateMany(ctx, indexes); err != nil {
		return err
	}

	logger.GetDebugLogger().Println("Ensured agentmods indexes")
	return nil
}

func ListForAgent(agentID bson.ObjectID) ([]v2.AgentModSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "modReference", Value: 1}})

	cur, err := collection().Find(ctx, bson.M{"agentId": agentID}, opts)
	if err != nil {
		return nil, err
	}

	mods := make([]v2.AgentModSchema, 0)
	if err := cur.All(ctx, &mods); err != nil {
		return nil, err
	}
	return mods, nil
}

// Get returns the agent's row for one mod, or (nil, nil) if it has none.
func Get(agentID bson.ObjectID, modReference string) (*v2.AgentModSchema, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	doc := &v2.AgentModSchema{}
	err := collection().FindOne(ctx, bson.M{"agentId": agentID, "modReference": modReference}).Decode(doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// UpsertMany writes a resolved lockfile back as the agent's selection: one
// upsert per mod, keyed on uniq_agent_mod. It sets desiredVersion, direct, and
// config, and leaves installed/installedVersion alone — those belong to the
// agent's report, and clobbering them here would make every sync look necessary.
func UpsertMany(agentID, accountID bson.ObjectID, locks []v2.ModLock, modIDs map[string]bson.ObjectID) error {
	if len(locks) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()
	writes := make([]mongo.WriteModel, 0, len(locks))

	for _, lock := range locks {
		modID, ok := modIDs[lock.ModReference]
		if !ok {
			return errors.New("no catalogue id for mod " + lock.ModReference)
		}

		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"agentId": agentID, "modReference": lock.ModReference}).
			SetUpdate(bson.M{
				"$set": bson.M{
					"desiredVersion": lock.Version,
					"direct":         lock.Direct,
					"config":         lock.Config,
					"modId":          modID,
					"updatedAt":      now,
				},
				"$setOnInsert": bson.M{
					"_id":              bson.NewObjectID(),
					"agentId":          agentID,
					"accountId":        accountID,
					"modReference":     lock.ModReference,
					"installed":        false,
					"installedVersion": "",
					"needsUpdate":      false,
					"latestVersion":    "",
					"createdAt":        now,
				},
			}).
			SetUpsert(true))
	}

	_, err := collection().BulkWrite(ctx, writes)
	return err
}

var errNilKeepList = errors.New("DeleteAbsent: nil keep list; refusing to delete every mod for the agent")

// deleteAbsentFilter builds the DeleteMany filter for DeleteAbsent, or refuses if
// keep is nil.
//
// nil and an explicit empty slice are NOT interchangeable here even though a
// $nin over either matches everything:
//   - keep == nil means the caller could not express any list at all - typically
//     an unchecked error from the lockfile resolution this feeds from. Passing that
//     straight through used to normalise nil to []string{}, which produced
//     modReference: {$nin: []}, matching every row for the agent and wiping the
//     agent's entire mod selection on what was really a transient failure. This is
//     exactly the bug the agentmods collection replaced (see the model's doc
//     comment in ssmcloud-resources/models/v2/agentmod.go): the old code deleted
//     files on disk for anything absent from a list a failed DB read could empty.
//   - keep == []string{} (non-nil, empty) means the resolver ran successfully and
//     concluded the user has zero mods left - e.g. they removed their last one.
//     That is a real outcome and must be allowed to delete every row, or the last
//     mod could never be removed.
//
// Do not "simplify" this by dropping the nil check: nil and []string{} look
// identical at the call site but mean opposite things.
func deleteAbsentFilter(agentID bson.ObjectID, keep []string) (bson.M, error) {
	if keep == nil {
		return nil, errNilKeepList
	}

	return bson.M{
		"agentId":      agentID,
		"modReference": bson.M{"$nin": keep},
	}, nil
}

// DeleteAbsent removes the agent's rows for mods no longer in the lockfile. It is
// how a removed mod, and a dependency nothing needs any more, leave the selection.
func DeleteAbsent(agentID bson.ObjectID, keep []string) error {
	filter, err := deleteAbsentFilter(agentID, keep)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = collection().DeleteMany(ctx, filter)
	return err
}

func SetDesiredVersion(agentID bson.ObjectID, modReference, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection().UpdateOne(ctx,
		bson.M{"agentId": agentID, "modReference": modReference},
		bson.M{"$set": bson.M{"desiredVersion": version, "updatedAt": time.Now()}})
	return err
}

func SetConfig(agentID bson.ObjectID, modReference, config string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection().UpdateOne(ctx,
		bson.M{"agentId": agentID, "modReference": modReference},
		bson.M{"$set": bson.M{"config": config, "updatedAt": time.Now()}})
	return err
}

// installedUpdate builds the write for one line of an agent's report. It is a
// separate function because what it must *not* write is the interesting part: the
// agent owns what is on disk and nothing else.
func installedUpdate(m v2.InstalledMod) bson.M {
	return bson.M{"$set": bson.M{
		"installed":        m.Installed,
		"installedVersion": m.InstalledVersion,
		"updatedAt":        time.Now(),
	}}
}

// ReportInstalled records what the agent actually has on disk. Mods the agent
// reports but which are not in its selection are ignored: the next sync removes
// them from the disk, and creating rows for them here would resurrect them.
func ReportInstalled(agentID bson.ObjectID, mods []v2.InstalledMod) error {
	if len(mods) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	writes := make([]mongo.WriteModel, 0, len(mods))
	for _, m := range mods {
		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"agentId": agentID, "modReference": m.ModReference}).
			SetUpdate(installedUpdate(m)))
	}

	_, err := collection().BulkWrite(ctx, writes)
	return err
}
