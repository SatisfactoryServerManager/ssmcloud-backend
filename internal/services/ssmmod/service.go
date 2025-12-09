package ssmmod

import (
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/bson"
)

func UpdateAgentPlayers(agentApiKey string, PostData types.API_UpdatePlayers_PostData) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := agent.GetAgentByAPIKey(agentApiKey)

	if err != nil {
		return err
	}

	// Update Existing Players
	for idx := range theAgent.MapData.Players {
		thePlayer := &theAgent.MapData.Players[idx]

		thePlayer.Online = false

		for _, apiPlayer := range PostData.Players {

			if thePlayer.Username == apiPlayer.Name {
				thePlayer.Online = true
				thePlayer.Location = apiPlayer.Location
			}
		}
	}

	for _, apiPlayer := range PostData.Players {
		foundPlayer := false
		for idx := range theAgent.MapData.Players {
			thePlayer := &theAgent.MapData.Players[idx]

			if thePlayer.Username == apiPlayer.Name {
				foundPlayer = true
				break
			}
		}

		if !foundPlayer {
			newPlayer := v2.AgentMapDataPlayer{
				Username: apiPlayer.Name,
				Location: apiPlayer.Location,
				Online:   true,
			}

			theAgent.MapData.Players = append(theAgent.MapData.Players, newPlayer)
		}
	}

	dbUpdate := bson.M{
		"mapData.players": theAgent.MapData.Players,
		"updatedAt":       time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func UpdateAgentBuildings(agentApiKey string, PostData types.API_UpdateBuildings_PostData) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := agent.GetAgentByAPIKey(agentApiKey)

	if err != nil {
		return err
	}

	newBuildingsArray := make([]v2.AgentMapDataBuilding, 0)

	// Update Existing Players
	for idx := range theAgent.MapData.Buildings {
		theBuilding := &theAgent.MapData.Buildings[idx]

		foundInApiData := false

		for _, apiBuilding := range PostData.Buildings {

			if theBuilding.Name == apiBuilding.Name {
				theBuilding.Location = apiBuilding.Location
				theBuilding.Rotation = apiBuilding.Rotation
				theBuilding.BoundingBox = apiBuilding.BoundingBox
				foundInApiData = true
			}
		}

		if foundInApiData {
			newBuildingsArray = append(newBuildingsArray, *theBuilding)
		}
	}

	theAgent.MapData.Buildings = newBuildingsArray

	for _, apiBuilding := range PostData.Buildings {
		foundBuilding := false
		for idx := range theAgent.MapData.Buildings {
			theBuilding := &theAgent.MapData.Buildings[idx]

			if theBuilding.Name == apiBuilding.Name {
				foundBuilding = true
				break
			}
		}

		if !foundBuilding {
			newBuilding := v2.AgentMapDataBuilding{
				Name:        apiBuilding.Name,
				Class:       apiBuilding.Class,
				Location:    apiBuilding.Location,
				Rotation:    apiBuilding.Rotation,
				BoundingBox: apiBuilding.BoundingBox,
			}

			theAgent.MapData.Buildings = append(theAgent.MapData.Buildings, newBuilding)
		}
	}

	dbUpdate := bson.M{
		"mapData.buildings": theAgent.MapData.Buildings,
		"updatedAt":         time.Now(),
	}

	if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}
