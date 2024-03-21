package services

import (
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
)

func UpdateAgentPlayers(agentApiKey string, PostData app.API_UpdatePlayers_PostData) error {

	theAgent, err := GetAgentByAPIKey(agentApiKey)

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
			newPlayer := models.AgentMapDataPlayer{
				Username: apiPlayer.Name,
				Location: apiPlayer.Location,
				Online:   true,
			}

			theAgent.MapData.Players = append(theAgent.MapData.Players, newPlayer)
		}
	}

	dbUpdate := bson.D{{"$set", bson.D{
		{"mapData.players", theAgent.MapData.Players},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theAgent, dbUpdate); err != nil {
		return err
	}

	return nil
}
