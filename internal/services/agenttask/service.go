package agenttask

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	collectionName = "agenttasks"
	modelName      = "AgentTask"

	LeaseDuration = 60 * time.Second
	finishedTTL   = 7 * 24 * time.Hour
)

func collection() *mongo.Collection {
	return repositories.GetMongoClient().GetCollection(collectionName)
}

func model() (*mongoose.Model, error) {
	return repositories.GetMongoClient().GetModel(modelName)
}

// EnsureIndexes creates the indexes the queue's correctness depends on.
//
// The two unique partial indexes are not optimizations. `uniq_running_per_agent`
// is what serializes tasks on an agent; `uniq_active_dedupe` is what makes an
// enqueue idempotent. Removing either reintroduces the double-install bug.
func EnsureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	col := collection()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}, {Key: "status", Value: 1}, {Key: "nextAttemptAt", Value: 1}},
			Options: options.Index().SetName("dispatch_claim"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "leaseExpiresAt", Value: 1}},
			Options: options.Index().SetName("reaper_sweep"),
		},
		{
			Keys: bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().
				SetName("uniq_running_per_agent").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"status": "running"}),
		},
		{
			Keys: bson.D{{Key: "agentId", Value: 1}, {Key: "dedupeKey", Value: 1}},
			Options: options.Index().
				SetName("uniq_active_dedupe").
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"active": bson.M{"$exists": true}}),
		},
		{
			Keys: bson.D{{Key: "finishedAt", Value: 1}},
			Options: options.Index().
				SetName("finished_ttl").
				SetExpireAfterSeconds(int32(finishedTTL.Seconds())),
		},
	}

	if _, err := col.Indexes().CreateMany(ctx, indexes); err != nil {
		return err
	}

	logger.GetDebugLogger().Println("Ensured agenttasks indexes")
	return nil
}
