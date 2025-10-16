package v2

import (
	"errors"
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetMyUserAccountAgents(theAccount *models.AccountSchema, agentId primitive.ObjectID) ([]*models.AgentSchema, error) {

	if theAccount == nil {
		emptyArray := make([]*models.AgentSchema, 0)
		return emptyArray, nil
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return nil, fmt.Errorf("error populating agents with error: %s", err.Error())
	}

	if !agentId.IsZero() {
		singleAgentArray := make([]*models.AgentSchema, 0)
		for idx := range theAccount.Agents {
			agent := &theAccount.Agents[idx]
			if agent.ID.Hex() == agentId.Hex() {
				singleAgentArray = append(singleAgentArray, agent)
			}
		}
		return singleAgentArray, nil
	}

	resArray := make([]*models.AgentSchema, 0)
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]
		resArray = append(resArray, agent)
	}
	return resArray, nil
}

func CreateAgentWorkflow(accountId primitive.ObjectID, PostData *models.CreateAgentWorkflowData) (string, error) {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return "", fmt.Errorf("error getting account model with error: %s", err.Error())
	}

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return "", fmt.Errorf("error getting workflow model with error: %s", err.Error())
	}

	theAccount := &models.AccountSchema{}

	if err := AccountModel.FindOneById(theAccount, accountId); err != nil {
		return "", fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return "", fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	for _, agent := range theAccount.Agents {
		if agent.AgentName == PostData.AgentName {
			return "", fmt.Errorf("error agent with same name %s already exists on your account", PostData.AgentName)
		}
	}

	PostData.AccountId = theAccount.ID

	createAgentAction := models.WorkflowAction{
		Type: models.WorkflowActionType_CreateAgent,
	}

	waitForOnlineAction := models.WorkflowAction{
		Type: models.WorkflowActionType_WaitForOnline,
	}

	installServerAction := models.WorkflowAction{
		Type: models.WorkflowActionType_InstallServer,
	}

	waitForInstalledAction := models.WorkflowAction{
		Type: models.WorkflowActionType_WaitForInstalled,
	}

	startServerAction := models.WorkflowAction{
		Type: models.WorkflowActionType_StartServer,
	}

	waitForRunningAction := models.WorkflowAction{
		Type: models.WorkflowActionType_WaitForRunning,
	}

	claimServerAction := models.WorkflowAction{
		Type: models.WorkflowActionType_ClaimServer,
	}

	workflow := models.WorkflowSchema{
		ID:   primitive.NewObjectID(),
		Type: models.WorkflowType_CreateAgent,
		Data: PostData,
		Actions: []models.WorkflowAction{
			createAgentAction,
			waitForOnlineAction,
			installServerAction,
			waitForInstalledAction,
			startServerAction,
			waitForRunningAction,
			claimServerAction,
		},
	}

	if err := WorkflowModel.Create(workflow); err != nil {
		return "", err
	}

	return workflow.ID.Hex(), nil
}

func DeleteAgent(theAccount *models.AccountSchema, agentId primitive.ObjectID) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return fmt.Errorf("error populating agents with error: %s", err.Error())
	}

	var theAgent *models.AgentSchema
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]
		if agent.ID.Hex() == agentId.Hex() {
			theAgent = agent
		}
	}

	if theAgent == nil {
		return errors.New("error finding agent with that id")
	}

	newAgents := make(primitive.A, 0)
	for _, agent := range theAccount.Agents {
		if agent.ID.Hex() != agentId.Hex() {
			newAgents = append(newAgents, agent.ID)
		}
	}

	theAccount.AgentIds = newAgents

	if err := AccountModel.UpdateData(theAccount, bson.M{"agents": theAccount.AgentIds}); err != nil {
		return fmt.Errorf("error removing agent from account with error: %s", err.Error())
	}

	if err := AgentModel.DeleteById(agentId); err != nil {
		return fmt.Errorf("error deleting agent from db with error: %s", err.Error())
	}

	return nil
}

func UpdateAgentSettings(theAccount *models.AccountSchema, PostData *app.APIUpdateServerSettingsRequest) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return fmt.Errorf("error getting account model with error: %s", err.Error())
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return fmt.Errorf("error getting agent model with error: %s", err.Error())
	}

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	oid, err := primitive.ObjectIDFromHex(PostData.ID)
	if err != nil {
		return err
	}

	var theAgent *models.AgentSchema
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]

		if agent.ID.Hex() == oid.Hex() {
			theAgent = agent
			break
		}
	}

	updateData := bson.M{}

	switch PostData.ConfigSetting {
	case "sfsettings":
		theAgent.ServerConfig.UpdateOnStart = (PostData.UpdateOnStart == "on")
		theAgent.ServerConfig.AutoRestart = (PostData.AutoRestart == "on")
		theAgent.ServerConfig.AutoPause = (PostData.AutoPause == "on")
		theAgent.ServerConfig.AutoSaveOnDisconnect = (PostData.AutoSaveOnDisconnect == "on")
		theAgent.ServerConfig.AutoSaveInterval = PostData.AutoSaveInterval
		theAgent.ServerConfig.DisableSeasonalEvents = PostData.SeasonalEvents != "on"
		theAgent.ServerConfig.MaxPlayers = PostData.MaxPlayers
		theAgent.ServerConfig.WorkerThreads = PostData.WorkerThreads
		if PostData.Branch == "on" {
			theAgent.ServerConfig.Branch = "experimental"
		} else {
			theAgent.ServerConfig.Branch = "public"
		}

		updateData["serverConfig"] = theAgent.ServerConfig

	case "backupsettings":
		theAgent.Config.BackupInterval = PostData.BackupInterval
		theAgent.Config.BackupKeepAmount = PostData.BackupKeep
		updateData["config"] = theAgent.Config
	default:
		return errors.New("error unknown config setting")
	}

	if len(updateData) == 0 {
		return nil
	}

	if err := AgentModel.UpdateData(theAgent, updateData); err != nil {
		return err
	}

	return nil
}

func GetAgentLog(theAgent *models.AgentSchema, logType string) (*models.AgentLogSchema, error) {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	if err := AgentModel.PopulateField(theAgent, "Logs"); err != nil {
		return nil, err
	}

	for idx := range theAgent.Logs {
		log := &theAgent.Logs[idx]
		if log.Type == logType {
			return log, nil
		}
	}
	return &models.AgentLogSchema{}, nil
}
