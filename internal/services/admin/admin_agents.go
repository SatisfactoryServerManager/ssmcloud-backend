package admin

import (
	"errors"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---- Agents ----

func AdminGetAgent(agentId string) (*models.AgentSchema, error) {
	if agentId == "" {
		return nil, errors.New("missing agent_id")
	}

	oid, err := bson.ObjectIDFromHex(agentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id")
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	theAgent := &models.AgentSchema{}
	if err := AgentModel.FindOneById(theAgent, oid); err != nil {
		return nil, err
	}

	return theAgent, nil
}

func AdminListAgents(page, pageSize int32, search string) ([]models.AgentSchema, int, error) {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, 0, err
	}

	agents := make([]models.AgentSchema, 0)
	filter := bson.M{}
	if search != "" {
		filter = bson.M{"$or": bson.A{
			bson.M{"agentName": bson.M{"$regex": search, "$options": "i"}},
			bson.M{"apiKey": bson.M{"$regex": search, "$options": "i"}},
		}}
	}

	if err := AgentModel.FindAll(&agents, filter); err != nil {
		return nil, 0, err
	}

	p, ps := normalizePaging(page, pageSize)
	paged, total := paginateSlice(agents, p, ps)
	return paged, total, nil
}

func AdminUpdateAgent(agentId, agentName, apiKey string) (*models.AgentSchema, error) {
	if agentId == "" {
		return nil, errors.New("missing agent_id")
	}

	oid, err := bson.ObjectIDFromHex(agentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id")
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return nil, err
	}

	theAgent := &models.AgentSchema{}
	if err := AgentModel.FindOneById(theAgent, oid); err != nil {
		return nil, err
	}

	update := bson.M{}
	if agentName != "" && agentName != theAgent.AgentName {
		theAgent.AgentName = agentName
		update["agentName"] = agentName
	}
	if apiKey != "" && apiKey != theAgent.APIKey {
		theAgent.APIKey = apiKey
		update["apiKey"] = apiKey
	}
	if len(update) == 0 {
		return theAgent, nil
	}

	theAgent.UpdatedAt = time.Now()
	update["updatedAt"] = theAgent.UpdatedAt

	if err := AgentModel.UpdateData(theAgent, update); err != nil {
		return nil, err
	}

	return theAgent, nil
}

func AdminDeleteAgent(agentId string) error {
	if agentId == "" {
		return errors.New("missing agent_id")
	}

	oid, err := bson.ObjectIDFromHex(agentId)
	if err != nil {
		return fmt.Errorf("invalid agent_id")
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	// Unlink from all accounts that reference this agent
	accounts := make([]models.AccountSchema, 0)
	_ = AccountModel.FindAll(&accounts, bson.M{"agents": bson.M{"$in": bson.A{oid}}})
	for i := range accounts {
		a := &accounts[i]
		newAgents := make(bson.A, 0)
		for _, aId := range a.AgentIds {
			if aOid, ok := aId.(bson.ObjectID); ok {
				if aOid.Hex() != oid.Hex() {
					newAgents = append(newAgents, aOid)
				}
			}
		}
		a.AgentIds = newAgents
		_ = AccountModel.UpdateData(a, bson.M{"agents": a.AgentIds, "updatedAt": time.Now()})
	}

	// Delete agent record (whether linked or not)
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}
	return AgentModel.DeleteById(oid)
}

// ---- Relationships ----
