package services

import (
	"errors"
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
		return emptyAgents, fmt.Errorf("error converting accountid to object id with error: %s", err.Error())
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
		return fmt.Errorf("error converting accountid to object id with error: %s", err.Error())
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

func GetAgentById(accountIdStr string, agentIdStr string) (models.Agents, error) {

	agents, err := GetAllAgents(accountIdStr)

	if err != nil {
		return models.Agents{}, err
	}

	agentId, err := primitive.ObjectIDFromHex(agentIdStr)

	if err != nil {
		return models.Agents{}, fmt.Errorf("error converting agentid to object id with error: %s", err.Error())
	}

	for _, agent := range agents {
		if agent.ID.Hex() == agentId.Hex() {
			return agent, nil
		}
	}

	return models.Agents{}, errors.New("error cant find agent on the account")
}

func GetAgentTasks(accountIdStr string, agentIdStr string) ([]models.AgentTask, error) {

	tasks := make([]models.AgentTask, 0)

	agent, err := GetAgentById(accountIdStr, agentIdStr)

	if err != nil {
		return tasks, err
	}

	return agent.Tasks, nil
}

func NewAgentTask(accountIdStr string, agentIdStr string, action string, data interface{}) error {

	newTask := models.NewAgentTask(action, data)

	agent, err := GetAgentById(accountIdStr, agentIdStr)

	if err != nil {
		return err
	}

	agent.Tasks = append(agent.Tasks, newTask)

	dbUpdate := bson.D{{"$set", bson.D{
		{"tasks", agent.Tasks},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&agent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func DeleteAgent(accountIdStr string, agentIdStr string) error {

	theAgent, err := GetAgentById(accountIdStr, agentIdStr)

	if err != nil {
		return err
	}

	account, err := GetAccount(accountIdStr)
	if err != nil {
		return err
	}

	if err := account.PopulateAgents(); err != nil {
		return fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	newAgentList := make(primitive.A, 0)

	for _, agent := range account.AgentObjects {
		if agent.ID.Hex() != theAgent.ID.Hex() {
			newAgentList = append(newAgentList, agent.ID)
		}
	}

	dbUpdate := bson.D{{"$set", bson.D{
		{"agents", newAgentList},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&account, dbUpdate); err != nil {
		return err
	}

	if _, err := mongoose.DeleteOne(bson.M{"_id": theAgent.ID}, "agents"); err != nil {
		return err
	}

	return nil
}
