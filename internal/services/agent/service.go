package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/integration"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	checkAllAgentsLastCommsJob *joblock.JobLockTask
	checkAgentVersionsJob      *joblock.JobLockTask
	uploadPendingLogsJob       *joblock.JobLockTask
)

func InitAgentService() {

	checkAllAgentsLastCommsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"checkAllAgentsLastCommsJob", func() {
			if err := CheckAllAgentsLastComms(); err != nil {
				logger.GetErrorLogger().Println(err)
			}
		},
		30*time.Second,
		1*time.Minute,
		false,
	)

	checkAgentVersionsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"checkAgentVersionsJob", func() {
			if err := CheckAgentVersions(); err != nil {
				logger.GetErrorLogger().Println(err)
			}
		},
		1*time.Minute,
		1*time.Minute,
		false,
	)

	ctx := context.Background()
	if err := checkAllAgentsLastCommsJob.Run(ctx); err != nil {
		logger.GetErrorLogger().Printf("%v", err.Error())
	}
	if err := checkAgentVersionsJob.Run(ctx); err != nil {
		logger.GetErrorLogger().Printf("%v", err.Error())
	}

	uploadPendingLogsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"uploadPendingLogsJob", func() {
			if err := UploadPendingLogs(); err != nil {
				logger.GetErrorLogger().Println(err)
			}
		},
		30*time.Second, // Run every 30 seconds
		1*time.Minute,  // Lock for 1 minute
		false,
	)

	if err := uploadPendingLogsJob.Run(ctx); err != nil {
		logger.GetErrorLogger().Printf("%v", err.Error())
	}
}

func ShutdownAgentService() error {
	ctx := context.Background()

	checkAllAgentsLastCommsJob.UnLock(ctx)
	checkAgentVersionsJob.UnLock(ctx)
	uploadPendingLogsJob.UnLock(ctx)

	logger.GetDebugLogger().Println("Shutdown Agent Service")
	return nil
}

func CheckAllAgentsLastComms() error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	allAgents := make([]modelsv2.AgentSchema, 0)

	if err := AgentModel.FindAll(&allAgents, bson.M{}); err != nil {
		return err
	}

	for idx := range allAgents {
		agent := &allAgents[idx]

		d := time.Now().Add(-1 * time.Hour)

		if agent.Status.LastCommDate.Before(d) {
			if agent.Status.Online {
				agent.Status.Online = false
				agent.Status.Running = false
				agent.Status.CPU = 0
				agent.Status.RAM = 0
				dbUpdate := bson.M{
					"status":    agent.Status,
					"updatedAt": time.Now(),
				}

				if err := AgentModel.UpdateData(agent, dbUpdate); err != nil {
					return err
				}
				theAccount := &modelsv2.AccountSchema{}
				filter := bson.M{"agents": bson.M{"$in": bson.A{agent.ID}}}

				if err := AccountModel.FindOne(theAccount, filter); err != nil {
					return fmt.Errorf("error finding account with error: %s", err.Error())
				}

				data := models.EventDataAgent{
					EventData: models.EventData{
						EventType: string(modelsv2.IntegrationEventTypeAgentOffline),
						EventTime: time.Now(),
					},
					AgentName: agent.AgentName,
				}

				if err := integration.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOffline, data); err != nil {
					return fmt.Errorf("error creating integration event with error: %s", err.Error())
				}
			}
		}
	}

	return nil
}

func CheckAgentVersions() error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	LatestVersion := os.Getenv("LATEST_AGENT_VERSION")

	allAgents := make([]modelsv2.AgentSchema, 0)

	if err := AgentModel.FindAll(&allAgents, bson.M{}); err != nil {
		return err
	}

	for idx := range allAgents {
		agent := &allAgents[idx]

		if agent.LatestAgentVersion != LatestVersion {
			agent.LatestAgentVersion = LatestVersion

			dbUpdate := bson.M{
				"latestAgentVersion": agent.LatestAgentVersion,
				"updatedAt":          time.Now(),
			}

			if err := AgentModel.UpdateData(agent, dbUpdate); err != nil {
				return fmt.Errorf("error updating account agents with error: %s", err.Error())
			}

		}
	}

	return nil
}
