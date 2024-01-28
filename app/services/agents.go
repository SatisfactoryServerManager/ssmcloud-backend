package services

import (
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetAllAgents(accountIdStr string) ([]models.Agents, error) {

	var theAccount models.Accounts
	emptyAgents := make([]models.Agents, 0)

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return emptyAgents, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return emptyAgents, fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateAgents(); err != nil {
		return emptyAgents, fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	return theAccount.AgentObjects, nil

}

func CreateAgent(accountIdStr string, agentName string, port int, memory int64) error {
	var theAccount models.Accounts

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateAgents(); err != nil {
		return fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	exists := false

	for _, agent := range theAccount.AgentObjects {
		if agent.AgentName == agentName {
			exists = true
			break
		}
	}

	if exists {
		return fmt.Errorf("error agent with same name %s already exists on your account", agentName)
	}

	newAgent := models.NewAgent(agentName, port, memory)

	if _, err := mongoose.InsertOne(&newAgent); err != nil {
		return fmt.Errorf("error inserting new agent with error: %s", err.Error())
	}

	theAccount.Agents = append(theAccount.Agents, newAgent.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"agents", theAccount.Agents},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theAccount, dbUpdate); err != nil {
		return fmt.Errorf("error updating account agents with error: %s", err.Error())
	}

	return nil
}