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
	"github.com/mircearoata/pubgrub-go/pubgrub/semver"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	checkAllAgentsLastCommsJob *joblock.JobLockTask
	purgeAgentTasksJob         *joblock.JobLockTask
	checkAgentModsConfigsJob   *joblock.JobLockTask
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

	purgeAgentTasksJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"purgeAgentTasksJob", func() {
			if err := PurgeAgentTasks(); err != nil {
				logger.GetErrorLogger().Println(err)
			}
		},
		30*time.Second,
		1*time.Minute,
		false,
	)

	checkAgentModsConfigsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"checkAgentModsConfigsJob", func() {
			if err := CheckAgentModsConfigs(); err != nil {
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
	if err := purgeAgentTasksJob.Run(ctx); err != nil {
		logger.GetErrorLogger().Printf("%v", err.Error())
	}
	if err := checkAgentModsConfigsJob.Run(ctx); err != nil {
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
	purgeAgentTasksJob.UnLock(ctx)
	checkAgentModsConfigsJob.UnLock(ctx)
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

func PurgeAgentTasks() error {

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

		if len(agent.Tasks) == 0 {
			continue
		}

		newTaskList := make([]modelsv2.AgentTask, 0)
		for _, task := range agent.Tasks {

			if task.Completed || task.Retries > 30 {
				continue
			}

			newTaskList = append(newTaskList, task)
		}

		if len(agent.Tasks) != len(newTaskList) {

			dbUpdate := bson.M{
				"tasks":     newTaskList,
				"updatedAt": time.Now(),
			}

			if err := AgentModel.UpdateData(agent, dbUpdate); err != nil {
				return err
			}
		}

	}

	return nil
}

func CheckAgentModsConfigs() error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	agents := make([]modelsv2.AgentSchema, 0)

	if err := AgentModel.FindAll(&agents, bson.M{}); err != nil {
		return fmt.Errorf("error finding agents with error: %s", err.Error())
	}

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return err
	}

	for idx := range agents {
		agent := &agents[idx]

		for modidx := range agent.ModConfig.SelectedMods {
			mod := &agent.ModConfig.SelectedMods[modidx]
			if err := ModModel.PopulateField(mod, "Mod"); err != nil {
				err = fmt.Errorf("error populating mod with error: %s", err.Error())
				return err
			}
		}
	}

	for idx := range agents {
		agent := &agents[idx]

		for modidx := range agent.ModConfig.SelectedMods {
			selectedMod := &agent.ModConfig.SelectedMods[modidx]

			if len(selectedMod.Mod.Versions) == 0 {
				continue
			}

			latestVersion, _ := semver.NewVersion(selectedMod.Mod.Versions[0].Version)

			//installedVersion, _ := semver.NewVersion(selectedMod.InstalledVersion)
			desiredVersion, _ := semver.NewVersion(selectedMod.DesiredVersion)

			if latestVersion.Compare(desiredVersion) == 0 {
				selectedMod.NeedsUpdate = false
			} else if latestVersion.Compare(desiredVersion) > 0 {
				selectedMod.NeedsUpdate = true
			}
		}

		dbUpdate := bson.M{
			"modConfig": agent.ModConfig,
			"updatedAt": time.Now(),
		}

		if err := AgentModel.UpdateData(agent, dbUpdate); err != nil {
			return err
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
