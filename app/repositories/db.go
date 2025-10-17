package repositories

import (
	"context"

	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose/mongoose"
)

var (
	db *mongoose.MongooseClient
)

func InitDBRepository() error {
	ctx := context.Background()
	mc, err := mongoose.NewMongoClient(ctx, mongoose.GetConnectionOptionsFromEnv())
	if err != nil {
		return err
	}

	db = mc

	err = registerModels(
		&models.UserSchema{},
		&models.AccountSchema{},
		&models.AccountAuditSchema{},
		&models.AgentSchema{},
		&models.WorkflowSchema{},
		&models.AgentLogSchema{},
		&models.AgentStatSchema{},
	)

	if err != nil {
		return err
	}

	return nil
}

func registerModels(schemas ...interface{}) error {
	for _, schema := range schemas {
		if _, err := db.RegisterModel(schema); err != nil {
			return err
		}
	}
	return nil
}

func GetMongoClient() *mongoose.MongooseClient {
	return db
}
