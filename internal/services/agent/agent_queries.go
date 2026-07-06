package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/integration"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetAllAgents(accountIdStr string) ([]*modelsv2.AgentSchema, error) {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	accountId, err := bson.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return nil, fmt.Errorf("error converting accountid to object id with error: %s", err.Error())
	}

	theAccount := &modelsv2.AccountSchema{}

	if err := AccountModel.FindOneById(theAccount, accountId); err != nil {
		return nil, fmt.Errorf("error finding account with error: %s", err.Error())
	}

	allAgents := make([]*modelsv2.AgentSchema, 0)

	if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
		return nil, fmt.Errorf("error populating account agents with error: %s", err.Error())
	}

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return nil, err
	}

	for idx := range theAccount.Agents {
		agent := &theAccount.Agents[idx]

		for modidx := range agent.ModConfig.SelectedMods {
			mod := &agent.ModConfig.SelectedMods[modidx]
			if err := ModModel.PopulateField(mod, "Mod"); err != nil {
				err = fmt.Errorf("error populating mod with error: %s", err.Error())
				return nil, err
			}
		}

		allAgents = append(allAgents, agent)
	}

	return allAgents, nil
}

func GetAgentById(accountIdStr string, agentIdStr string) (*modelsv2.AgentSchema, error) {

	agents, err := GetAllAgents(accountIdStr)

	if err != nil {
		return nil, err
	}

	agentId, err := bson.ObjectIDFromHex(agentIdStr)

	if err != nil {
		return nil, fmt.Errorf("error converting agentid to object id with error: %s", err.Error())
	}

	for _, agent := range agents {
		if agent.ID.Hex() == agentId.Hex() {
			return agent, nil
		}
	}

	return nil, errors.New("error cant find agent on the account")
}

func GetAgentByIdNoAccount(agentIdStr string) (*modelsv2.AgentSchema, error) {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	agents := make([]modelsv2.AgentSchema, 0)

	if err := AgentModel.FindAll(&agents, bson.M{}); err != nil {
		return nil, err
	}

	if len(agentIdStr) < 8 {
		return nil, errors.New("invalid agent id")
	}

	if len(agentIdStr) == 8 {
		for idx := range agents {
			agent := &agents[idx]
			if strings.HasSuffix(agent.ID.Hex(), agentIdStr) {
				return agent, nil
			}
		}
	} else {

		agentId, err := bson.ObjectIDFromHex(agentIdStr)

		if err != nil {
			return nil, fmt.Errorf("error converting agentid to object id with error: %s", err.Error())
		}

		for idx := range agents {
			agent := &agents[idx]
			if agent.ID.Hex() == agentId.Hex() {
				return agent, nil
			}
		}
	}

	return nil, errors.New("error cant find agent")
}

// Agent API Functions

func GetAgentByAPIKey(agentAPIKey string) (*modelsv2.AgentSchema, error) {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	theAgent := &modelsv2.AgentSchema{}

	if err := AgentModel.FindOne(theAgent, bson.M{"apiKey": agentAPIKey}); err != nil {
		return theAgent, err
	}

	return theAgent, nil
}

func UpdateAgentLastComm(agentAPIKey string) error {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	theAgent.Status.LastCommDate = time.Now()

	dbUpdate := bson.M{
		"status":    theAgent.Status,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func UpdateAgentStatus(agentAPIKey string, online bool, installed bool, running bool, cpu float64, mem float32, installedSFVersion int64, latestSFVersion int64) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	theAccount := &modelsv2.AccountSchema{}
	filter := bson.M{"agents": bson.M{"$in": bson.A{theAgent.ID}}}

	if err := AccountModel.FindOne(theAccount, filter); err != nil {
		return fmt.Errorf("error finding account with error: %s", err.Error())
	}

	if !theAgent.Status.Online && online {

		data := models.EventDataAgent{
			EventData: models.EventData{
				EventType: string(modelsv2.IntegrationEventTypeAgentOnline),
				EventTime: time.Now(),
			},
			AgentName: theAgent.AgentName,
		}

		if err := integration.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOnline, data); err != nil {
			return fmt.Errorf("error creating integration event with error: %s", err.Error())
		}
	} else if theAgent.Status.Online && !online {

		data := models.EventDataAgent{
			EventData: models.EventData{
				EventType: "agent.offline",
				EventTime: time.Now(),
			},
			AgentName: theAgent.AgentName,
		}

		if err := integration.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOffline, data); err != nil {
			return fmt.Errorf("error creating integration event with error: %s", err.Error())
		}
	}

	if err := AddAgentStat(theAgent, running, cpu, mem); err != nil {
		return err
	}

	if err := PurgeAgentStats(); err != nil {
		return err
	}

	theAgent.Status.Online = online
	theAgent.Status.Installed = installed
	theAgent.Status.Running = running
	theAgent.Status.CPU = cpu
	theAgent.Status.RAM = float64(mem)
	theAgent.Status.InstalledSFVersion = installedSFVersion
	theAgent.Status.LatestSFVersion = latestSFVersion

	dbUpdate := bson.M{
		"status":    theAgent.Status,
		"updatedAt": time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}
