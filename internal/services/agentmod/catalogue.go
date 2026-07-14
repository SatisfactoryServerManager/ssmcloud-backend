package agentmod

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/mod/semver"
)

// newestVersion is the catalogue's highest version of a mod. semver.Compare needs
// a leading v, and the catalogue's version strings are not guaranteed to carry
// one already (see withV in resolve.go). The catalogue's Versions order is not
// guaranteed either, so this cannot just take the first element - which is what
// the code this replaces did.
func newestVersion(versions []models.ModVersion) string {
	newest := ""

	for _, v := range versions {
		// withV (defined in resolve.go) MUST be used on both sides here: the
		// catalogue's version strings are not guaranteed bare, and "v"+v.Version
		// double-prefixes an already-prefixed one (e.g. "v3.10.0" -> "vv3.10.0"),
		// which semver.Compare treats as invalid. Invalid-vs-invalid compares
		// equal, so newest would never advance past the first element again -
		// exactly the Versions[0] bug this function exists to replace.
		if newest == "" || semver.Compare(withV(v.Version), withV(newest)) > 0 {
			newest = v.Version
		}
	}

	return newest
}

// RefreshNeedsUpdate flags every agent mod whose catalogue version has moved past
// its pin. It never touches desiredVersion and never enqueues a task: a version
// bump is always a user action, so a bad mod release cannot take down a live
// server unattended.
//
// This replaces CheckAgentModsConfigs, which loaded every agent document in the
// database and wrote every mod config back to compute this boolean. Here it is
// one updateMany per catalogue mod, keyed on modReference, never touching an
// agent document at all.
func RefreshNeedsUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cur, err := repositories.GetMongoClient().GetCollection("mods").
		Find(ctx, bson.M{}, nil)
	if err != nil {
		return err
	}

	catalogue := make([]models.ModSchema, 0)
	if err := cur.All(ctx, &catalogue); err != nil {
		return err
	}

	writes := make([]mongo.WriteModel, 0)
	now := time.Now()

	for idx := range catalogue {
		m := &catalogue[idx]

		latest := newestVersion(m.Versions)
		if latest == "" {
			continue
		}

		// needsUpdate is "the catalogue has moved past what this agent pinned".
		// The $set uses an aggregation-pipeline update so it can be computed from
		// the document's own desiredVersion in a single round trip. A string
		// inequality is the right test here rather than a semver comparison: the
		// pin is either exactly the catalogue's newest version or it is not.
		writes = append(writes, mongo.NewUpdateManyModel().
			SetFilter(bson.M{"modReference": m.ModReference}).
			SetUpdate([]bson.M{{
				"$set": bson.M{
					"latestVersion": latest,
					"needsUpdate": bson.M{
						"$ne": bson.A{"$desiredVersion", latest},
					},
					"updatedAt": now,
				},
			}}))
	}

	if len(writes) == 0 {
		return nil
	}

	// Unordered: each updateMany is keyed on a distinct modReference, so one bad
	// catalogue mod must not abort the rest of the batch and leave their
	// needsUpdate flags stale until the next tick.
	res, err := collection().BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return err
	}

	logger.GetDebugLogger().Printf("refreshed needsUpdate on %d agent mods", res.ModifiedCount)
	return nil
}
