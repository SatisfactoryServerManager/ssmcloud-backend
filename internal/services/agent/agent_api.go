package agent

import (
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/audit"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/integration"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetUserAccountAgents(theAccount *modelsv2.AccountSchema, agentId bson.ObjectID) ([]*modelsv2.AgentSchema, error) {

	if theAccount == nil {
		emptyArray := make([]*modelsv2.AgentSchema, 0)
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
		singleAgentArray := make([]*modelsv2.AgentSchema, 0)
		for idx := range theAccount.Agents {
			agent := &theAccount.Agents[idx]
			if agent.ID.Hex() == agentId.Hex() {
				singleAgentArray = append(singleAgentArray, agent)
			}
		}

		if len(singleAgentArray) == 0 {
			return nil, errors.New("error cant find agent with id")
		}
		return singleAgentArray, nil
	}

	resArray := make([]*modelsv2.AgentSchema, 0)
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]
		resArray = append(resArray, agent)
	}
	return resArray, nil
}

func NewWorkflow_CreateAgent(accountId bson.ObjectID, PostData *modelsv2.CreateAgentWorkflowData) (string, error) {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return "", fmt.Errorf("error getting account model with error: %s", err.Error())
	}

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return "", fmt.Errorf("error getting workflow model with error: %s", err.Error())
	}

	theAccount := &modelsv2.AccountSchema{}

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

	createAgentAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_CreateAgent,
	}

	waitForOnlineAction := modelsv2.WorkflowAction{
		Type: modelsv2.WorkflowActionType_WaitForOnline,
	}

	installServerAction := modelsv2.WorkflowAction{
		Type:       modelsv2.WorkflowActionType_AgentTask,
		TaskAction: "installsfserver",
		Timeout:    30 * time.Minute,
	}

	startServerAction := modelsv2.WorkflowAction{
		Type:       modelsv2.WorkflowActionType_AgentTask,
		TaskAction: "startsfserver",
		Timeout:    10 * time.Minute,
	}

	claimServerAction := modelsv2.WorkflowAction{
		Type:       modelsv2.WorkflowActionType_AgentTask,
		TaskAction: "claimserver",
		TaskData: modelsv2.ClaimServer_PostData{
			AdminPass:  PostData.AdminPass,
			ClientPass: PostData.ClientPass,
		},
		Timeout: 10 * time.Minute,
	}

	workflow := modelsv2.WorkflowSchema{
		ID:   bson.NewObjectID(),
		Type: modelsv2.WorkflowType_CreateAgent,
		Data: PostData,
		Actions: []modelsv2.WorkflowAction{
			createAgentAction,
			waitForOnlineAction,
			installServerAction,
			startServerAction,
			claimServerAction,
		},
	}

	if err := WorkflowModel.Create(workflow); err != nil {
		return "", err
	}

	return workflow.ID.Hex(), nil
}

func DeleteAgent(theAccount *modelsv2.AccountSchema, agentId bson.ObjectID) error {

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

	var theAgent *modelsv2.AgentSchema
	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]
		if agent.ID.Hex() == agentId.Hex() {
			theAgent = agent
		}
	}

	if theAgent == nil {
		return errors.New("error finding agent with that id")
	}

	newAgents := make(bson.A, 0)
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

	if err := audit.AddAccountAudit(theAccount,
		modelsv2.AuditType_AgentRemoveFromAccount,
		fmt.Sprintf("Agent (%s) was removed from the account", theAgent.AgentName),
	); err != nil {
		return err
	}

	if err := integration.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentRemoved, models.EventDataAgent{
		AgentName: theAgent.AgentName,
	}); err != nil {
		return err
	}

	return nil
}

func UpdateAgentSettings(theAgent *modelsv2.AgentSchema, PostData *types.APIUpdateServerSettings) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return fmt.Errorf("error getting agent model with error: %s", err.Error())
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

func GetAgentLog(theAgent *modelsv2.AgentSchema, logType string) (*modelsv2.AgentLogSchema, error) {
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
	return &modelsv2.AgentLogSchema{}, nil
}

// CreateAgentTask enqueues a user-triggered task and returns its id.
func CreateAgentTask(theAgent *modelsv2.AgentSchema, theAccount *modelsv2.AccountSchema, externalID, action string, data interface{}) (string, error) {
	return agenttask.Enqueue(
		theAgent.ID,
		theAccount.ID,
		action,
		data,
		"", // user-triggered actions are not deduped: asking twice means twice
		modelsv2.TaskTrigger{Type: modelsv2.TaskTriggerUser, ExternalID: externalID},
		agenttask.EnqueueOpts{},
	)
}

func AddAgentStat(theAgent *modelsv2.AgentSchema, runningState bool, cpu float64, memory float32) error {

	AgentStatModel, err := repositories.GetMongoClient().GetModel("AgentStat")
	if err != nil {
		return err
	}
	newStat := modelsv2.NewAgentStat(theAgent, runningState, cpu, memory)

	if err := AgentStatModel.Create(newStat); err != nil {
		return fmt.Errorf("error creating agent stat with error: %s", err.Error())
	}

	return nil
}

func PurgeAgentStats() error {

	now := time.Now()
	expiry := now.AddDate(0, 0, -1)

	AgentStatModel, err := repositories.GetMongoClient().GetModel("AgentStat")
	if err != nil {
		return err
	}

	filter := bson.M{"createdAt": bson.M{"$lt": expiry}}

	if err := AgentStatModel.Delete(filter); err != nil {
		return fmt.Errorf("error deleting agent stats with error: %s", err.Error())
	}

	return nil
}

func GetAgentStats(theAgent *modelsv2.AgentSchema) ([]*modelsv2.AgentStatSchema, error) {

	AgentStatModel, err := repositories.GetMongoClient().GetModel("AgentStat")
	if err != nil {
		return nil, err
	}

	stats := make([]*modelsv2.AgentStatSchema, 0)
	if err := AgentStatModel.FindAll(&stats, bson.M{"agentId": theAgent.ID}); err != nil {
		return nil, err
	}

	return stats, nil

}
