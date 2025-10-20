package repositories

import (
	"context"

	"github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
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
		&v2.UserSchema{},
		&v2.AccountSchema{},
		&v2.AccountAuditSchema{},
		&v2.AgentSchema{},
		&v2.WorkflowSchema{},
		&v2.AgentLogSchema{},
		&v2.AgentStatSchema{},
		&v2.AgentModConfigSelectedMod{},
		&models.Mods{},
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
