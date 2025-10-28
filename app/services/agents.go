package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv1 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/google/go-github/github"
	"github.com/mircearoata/pubgrub-go/pubgrub/semver"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	checkAllAgentsLastCommsJob *joblock.JobLockTask
	purgeAgentTasksJob         *joblock.JobLockTask
	checkAgentModsConfigsJob   *joblock.JobLockTask
	checkAgentVersionsJob      *joblock.JobLockTask
)

func InitAgentService() {

	checkAllAgentsLastCommsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"checkAllAgentsLastCommsJob", func() {
			if err := CheckAllAgentsLastComms(); err != nil {
				fmt.Println(err)
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
				fmt.Println(err)
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
				fmt.Println(err)
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
				fmt.Println(err)
			}
		},
		30*time.Minute,
		1*time.Minute,
		false,
	)

	ctx := context.Background()
	if err := checkAllAgentsLastCommsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
	if err := purgeAgentTasksJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
	if err := checkAgentModsConfigsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
	if err := checkAgentVersionsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
}

func ShutdownAgentService() error {
	ctx := context.Background()

	checkAllAgentsLastCommsJob.UnLock(ctx)
	purgeAgentTasksJob.UnLock(ctx)
	checkAgentModsConfigsJob.UnLock(ctx)
	checkAgentVersionsJob.UnLock(ctx)

	logger.GetDebugLogger().Println("Shutdown Agent Service")
	return nil
}

func CheckAllAgentsLastComms() error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	allAgents := make([]modelsv1.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &allAgents); err != nil {
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

				if err := mongoose.UpdateModelData(agent, dbUpdate); err != nil {
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

				if err := v2.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOffline, data); err != nil {
					return fmt.Errorf("error creating integration event with error: %s", err.Error())
				}
			}
		}
	}

	return nil
}

func PurgeAgentTasks() error {

	allAgents := make([]modelsv1.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &allAgents); err != nil {
		return err
	}

	for idx := range allAgents {
		agent := &allAgents[idx]

		if err := agent.PurgeTasks(); err != nil {
			return err
		}

	}

	return nil
}

func CheckAgentModsConfigs() error {

	agents := make([]modelsv1.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &agents); err != nil {
		return fmt.Errorf("error finding agents with error: %s", err.Error())
	}

	for idx := range agents {
		agent := &agents[idx]

		agent.PopulateModConfig()

		for modidx := range agent.ModConfig.SelectedMods {
			selectedMod := &agent.ModConfig.SelectedMods[modidx]

			if len(selectedMod.ModObject.Versions) == 0 {
				continue
			}

			latestVersion, _ := semver.NewVersion(selectedMod.ModObject.Versions[0].Version)

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

		if err := mongoose.UpdateModelData(agent, dbUpdate); err != nil {
			return err
		}
	}

	return nil
}

func CheckAgentVersions() error {

	ctx := context.Background()

	client := github.NewClient(nil)
	opt := &github.ListOptions{Page: 1, PerPage: 10}
	releases, _, err := client.Repositories.ListReleases(ctx, "SatisfactoryServerManager", "SSMAgent", opt)

	if err != nil {
		return err
	}

	if len(releases) == 0 {
		return fmt.Errorf("error releases returned empty array")
	}

	LatestVersion := releases[0].TagName

	allAgents := make([]modelsv1.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &allAgents); err != nil {
		return err
	}

	for idx := range allAgents {
		agent := &allAgents[idx]

		if agent.LatestAgentVersion != *LatestVersion {
			agent.LatestAgentVersion = *LatestVersion

			dbUpdate := bson.M{
				"latestAgentVersion": agent.LatestAgentVersion,
				"updatedAt":          time.Now(),
			}

			if err := mongoose.UpdateModelData(agent, dbUpdate); err != nil {
				return fmt.Errorf("error updating account agents with error: %s", err.Error())
			}

		}
	}

	return nil
}

func GetAllAgents(accountIdStr string) ([]modelsv1.Agents, error) {

	var theAccount modelsv1.Accounts
	emptyAgents := make([]modelsv1.Agents, 0)

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

	for idx := range theAccount.AgentObjects {
		agent := &theAccount.AgentObjects[idx]

		agent.PopulateModConfig()
	}

	return theAccount.AgentObjects, nil
}

func GetAgentById(accountIdStr string, agentIdStr string) (modelsv1.Agents, error) {

	agents, err := GetAllAgents(accountIdStr)

	if err != nil {
		return modelsv1.Agents{}, err
	}

	agentId, err := primitive.ObjectIDFromHex(agentIdStr)

	if err != nil {
		return modelsv1.Agents{}, fmt.Errorf("error converting agentid to object id with error: %s", err.Error())
	}

	for _, agent := range agents {
		if agent.ID.Hex() == agentId.Hex() {
			return agent, nil
		}
	}

	return modelsv1.Agents{}, errors.New("error cant find agent on the account")
}

func GetAgentByIdNoAccount(agentIdStr string) (modelsv1.Agents, error) {

	agents := make([]modelsv1.Agents, 0)

	if err := mongoose.FindAll(bson.M{}, &agents); err != nil {
		return modelsv1.Agents{}, err
	}

	if len(agentIdStr) < 8 {
		return modelsv1.Agents{}, errors.New("invalid agent id")
	}

	if len(agentIdStr) == 8 {
		for _, agent := range agents {
			if strings.HasSuffix(agent.ID.Hex(), agentIdStr) {
				return agent, nil
			}
		}
	} else {

		agentId, err := primitive.ObjectIDFromHex(agentIdStr)

		if err != nil {
			return modelsv1.Agents{}, fmt.Errorf("error converting agentid to object id with error: %s", err.Error())
		}

		for _, agent := range agents {
			if agent.ID.Hex() == agentId.Hex() {
				return agent, nil
			}
		}
	}

	return modelsv1.Agents{}, errors.New("error cant find agent")
}

// Agent API Functions

func GetAgentByAPIKey(agentAPIKey string) (modelsv1.Agents, error) {

	var theAgent modelsv1.Agents

	if err := mongoose.FindOne(bson.M{"apiKey": agentAPIKey}, &theAgent); err != nil {
		return theAgent, err
	}

	return theAgent, nil
}

func UpdateAgentLastComm(agentAPIKey string) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	agent.Status.LastCommDate = time.Now()

	dbUpdate := bson.M{
		"status":    agent.Status,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	return nil
}

func UpdateAgentStatus(agentAPIKey string, online bool, installed bool, running bool, cpu float64, mem float32, installedSFVersion int64, latestSFVersion int64) error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	theAccount := &modelsv2.AccountSchema{}
	filter := bson.M{"agents": bson.M{"$in": bson.A{agent.ID}}}

	if err := AccountModel.FindOne(theAccount, filter); err != nil {
		return fmt.Errorf("error finding account with error: %s", err.Error())
	}

	if !agent.Status.Online && online {

		data := models.EventDataAgent{
			EventData: models.EventData{
				EventType: string(modelsv2.IntegrationEventTypeAgentOnline),
				EventTime: time.Now(),
			},
			AgentName: agent.AgentName,
		}

		if err := v2.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOnline, data); err != nil {
			return fmt.Errorf("error creating integration event with error: %s", err.Error())
		}
	} else if agent.Status.Online && !online {

		data := models.EventDataAgent{
			EventData: models.EventData{
				EventType: "agent.offline",
				EventTime: time.Now(),
			},
			AgentName: agent.AgentName,
		}

		if err := v2.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOffline, data); err != nil {
			return fmt.Errorf("error creating integration event with error: %s", err.Error())
		}
	}

	if err := agent.CreateStat(running, cpu, mem); err != nil {
		return err
	}

	if err := agent.PurgeStats(); err != nil {
		return err
	}

	agent.Status.Online = online
	agent.Status.Installed = installed
	agent.Status.Running = running
	agent.Status.CPU = cpu
	agent.Status.RAM = float64(mem)
	agent.Status.InstalledSFVersion = installedSFVersion
	agent.Status.LatestSFVersion = latestSFVersion

	dbUpdate := bson.M{
		"status":    agent.Status,
		"stats":     agent.Stats,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func UploadedAgentSave(agentAPIKey string, fileIdentity types.StorageFileIdentity, updateModTime bool) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	var theAccount modelsv1.Accounts
	if err := mongoose.FindOne(bson.M{"agents": agent.ID}, &theAccount); err != nil {
		return fmt.Errorf("error finding agent account with error: %s", err.Error())
	}
	objectPath := fmt.Sprintf("%s/%s/saves/%s", theAccount.ID.Hex(), agent.ID.Hex(), fileIdentity.FileName)

	objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
	if err != nil {
		return fmt.Errorf("error uploading file to minio with error: %s", err)
	}

	agentSaveExists := false

	for _, save := range agent.Saves {
		if save.FileName == fileIdentity.FileName {
			agentSaveExists = true
			break
		}
	}

	if !agentSaveExists {
		newAgentSave := modelsv1.AgentSave{
			UUID:      fileIdentity.UUID,
			FileName:  fileIdentity.FileName,
			FileUrl:   objectUrl,
			Size:      fileIdentity.Filesize,
			CreatedAt: time.Now(),
		}

		if updateModTime {
			newAgentSave.ModTime = time.Now()
		}

		agent.Saves = append(agent.Saves, newAgentSave)
	} else {
		for idx := range agent.Saves {
			save := &agent.Saves[idx]
			if save.FileName == fileIdentity.FileName {
				save.Size = fileIdentity.Filesize
				save.UpdatedAt = time.Now()

				if updateModTime {
					save.ModTime = time.Now()
				}
			}
		}
	}

	dbUpdate := bson.M{
		"saves":     agent.Saves,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func UploadedAgentBackup(agentAPIKey string, fileIdentity types.StorageFileIdentity) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	var theAccount modelsv1.Accounts
	if err := mongoose.FindOne(bson.M{"agents": agent.ID}, &theAccount); err != nil {
		return fmt.Errorf("error finding agent account with error: %s", err.Error())
	}

	objectPath := fmt.Sprintf("%s/%s/backups/%s", theAccount.ID.Hex(), agent.ID.Hex(), fileIdentity.FileName)

	objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
	if err != nil {
		return fmt.Errorf("error uploading file to minio with error: %s", err)
	}

	newAgentBackup := modelsv1.AgentBackup{
		UUID:      fileIdentity.UUID,
		FileName:  fileIdentity.FileName,
		Size:      fileIdentity.Filesize,
		FileUrl:   objectUrl,
		CreatedAt: time.Now(),
	}

	agent.Backups = append(agent.Backups, newAgentBackup)

	dbUpdate := bson.M{
		"backups":   agent.Backups,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func UploadedAgentLog(agentAPIKey string, fileIdentity types.StorageFileIdentity) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	var theAccount modelsv1.Accounts
	if err := mongoose.FindOne(bson.M{"agents": agent.ID}, &theAccount); err != nil {
		return fmt.Errorf("error finding agent account with error: %s", err.Error())
	}

	fileContents, err := utils.ReadLastNBtyesFromFile(fileIdentity.LocalFilePath, 4000)

	if err != nil {
		return fmt.Errorf("error reading log contents with error: %s", err.Error())
	}

	objectPath := fmt.Sprintf("%s/%s/logs/%s", theAccount.ID.Hex(), agent.ID.Hex(), fileIdentity.FileName)

	objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
	if err != nil {
		return fmt.Errorf("error uploading file to minio with error: %s", err)
	}

	if err := agent.PopulateLogs(); err != nil {
		return fmt.Errorf("error populating agent logs with error: %s", err.Error())
	}

	logType := "FactoryGame"

	if strings.HasPrefix(fileIdentity.FileName, "SSMAgent") {
		logType = "Agent"
	}
	if strings.HasPrefix(fileIdentity.FileName, "Steam") {
		logType = "Steam"
	}

	var theLog modelsv1.AgentLogs
	hasLog := false

	for _, log := range agent.LogObjects {
		if log.Type == logType {
			hasLog = true
			theLog = log
			break
		}
	}

	if !hasLog {
		theLog := modelsv1.AgentLogs{
			ID:        primitive.NewObjectID(),
			FileName:  fileIdentity.FileName,
			Type:      logType,
			Snippet:   fileContents,
			FileURL:   objectUrl,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if _, err := mongoose.InsertOne(&theLog); err != nil {
			return fmt.Errorf("error inserting new agent log with error: %s", err.Error())
		}

		agent.Logs = append(agent.Logs, theLog.ID)

		dbUpdate := bson.M{
			"logs":      agent.Logs,
			"updatedAt": time.Now(),
		}

		if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
			return err
		}
	}

	theLog.Snippet = fileContents
	theLog.FileName = fileIdentity.FileName
	theLog.FileURL = objectUrl

	dbUpdate := bson.M{
		"snippet":   theLog.Snippet,
		"fileName":  theLog.FileName,
		"updatedAt": time.Now(),
		"fileUrl":   theLog.FileURL,
	}

	if err := mongoose.UpdateModelData(&theLog, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func GetAgentModConfig(agentAPIKey string) (modelsv1.AgentModConfig, error) {

	var theModConfig modelsv1.AgentModConfig

	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return theModConfig, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	agent.PopulateModConfig()

	return agent.ModConfig, nil

}

func UpdateAgentModConfig(agentAPIKey string, newConfig modelsv1.AgentModConfig) error {

	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	agent.PopulateModConfig()

	for idx := range agent.ModConfig.SelectedMods {
		agentMod := &agent.ModConfig.SelectedMods[idx]

		for newIdx := range newConfig.SelectedMods {
			newMod := newConfig.SelectedMods[newIdx]

			if newMod.ModObject.ModReference == agentMod.ModObject.ModReference {
				agentMod.Installed = newMod.Installed
				agentMod.InstalledVersion = newMod.InstalledVersion
				agentMod.Config = newMod.Config
			}
		}
	}

	dbUpdate := bson.M{
		"modConfig": agent.ModConfig,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func GetAgentTasksApi(agentAPIKey string) ([]modelsv1.AgentTask, error) {
	tasks := make([]modelsv1.AgentTask, 0)

	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return tasks, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	if err := agent.PurgeTasks(); err != nil {
		return tasks, err
	}

	return agent.Tasks, nil
}

func UpdateAgentTaskItem(agentAPIKey string, taskId string, newTask modelsv1.AgentTask) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	if err := agent.PurgeTasks(); err != nil {
		return err
	}

	for idx := range agent.Tasks {
		task := &agent.Tasks[idx]

		if task.ID.Hex() != newTask.ID.Hex() {
			continue
		}

		task.Completed = newTask.Completed
		task.Retries = newTask.Retries
	}

	dbUpdate := bson.M{
		"tasks":     agent.Tasks,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func GetAgentConfig(agentAPIKey string) (app.API_AgentConfig_ResData, error) {
	var config app.API_AgentConfig_ResData

	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return config, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	config.Config = agent.Config
	config.ServerConfig = agent.ServerConfig

	return config, nil
}

func GetAgentSaves(agentAPIKey string) ([]modelsv1.AgentSave, error) {
	saves := make([]modelsv1.AgentSave, 0)
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return saves, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	saves = agent.Saves

	return saves, nil

}

func PostAgentSyncSaves(agentAPIKey string, saves []modelsv1.AgentSave) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	newSavesList := make([]modelsv1.AgentSave, 0)

	hasChanged := false

	// Check for new save info
	for updateIdx := range saves {
		updatedSave := &saves[updateIdx]

		found := false

		for agentSaveIdx := range agent.Saves {
			agentSave := &agent.Saves[agentSaveIdx]

			if updatedSave.FileName == agentSave.FileName {

				if agentSave.ModTime != updatedSave.ModTime {
					agentSave.Size = updatedSave.Size
					agentSave.ModTime = updatedSave.ModTime
					hasChanged = true
				}

				newSavesList = append(newSavesList, *agentSave)
				found = true
				break
			}
		}

		if !found {
			newSavesList = append(newSavesList, *updatedSave)
			hasChanged = true
		}
	}

	if hasChanged {
		dbUpdate := bson.M{
			"saves":     newSavesList,
			"updatedAt": time.Now(),
		}

		if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
			return err
		}

	}

	return nil
}

func UpdateAgentConfigApi(agentAPIKey string, version string, ip string) error {
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	agent.Config.Version = version
	agent.Config.IP = ip

	dbUpdate := bson.M{
		"config.version": agent.Config.Version,
		"config.ip":      agent.Config.IP,
		"updatedAt":      time.Now(),
	}

	if err := mongoose.UpdateModelData(&agent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}
