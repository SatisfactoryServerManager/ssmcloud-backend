package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/google/go-github/github"
	"github.com/mircearoata/pubgrub-go/pubgrub/semver"
	"github.com/mrhid6/go-mongoose-lock/joblock"
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

				if err := v2.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOffline, data); err != nil {
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

			if task.Completed {
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

	allAgents := make([]modelsv2.AgentSchema, 0)

	if err := AgentModel.FindAll(&allAgents, bson.M{}); err != nil {
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

			if err := AgentModel.UpdateData(agent, dbUpdate); err != nil {
				return fmt.Errorf("error updating account agents with error: %s", err.Error())
			}

		}
	}

	return nil
}

func GetAllAgents(accountIdStr string) ([]*modelsv2.AgentSchema, error) {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

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

	agentId, err := primitive.ObjectIDFromHex(agentIdStr)

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

		agentId, err := primitive.ObjectIDFromHex(agentIdStr)

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

		if err := v2.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOnline, data); err != nil {
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

		if err := v2.AddIntegrationEvent(theAccount, modelsv2.IntegrationEventTypeAgentOffline, data); err != nil {
			return fmt.Errorf("error creating integration event with error: %s", err.Error())
		}
	}

	if err := v2.AddAgentStat(theAgent, running, cpu, mem); err != nil {
		return err
	}

	if err := v2.PurgeAgentStats(); err != nil {
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

func UploadedAgentSave(agentAPIKey string, fileIdentity types.StorageFileIdentity, updateModTime bool) error {
	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAccount := &modelsv2.AccountSchema{}
	filter := bson.M{"agents": bson.M{"$in": bson.A{theAgent.ID}}}

	if err := AccountModel.FindOne(theAccount, filter); err != nil {
		return fmt.Errorf("error finding account with error: %s", err.Error())
	}

	objectPath := fmt.Sprintf("%s/%s/saves/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), fileIdentity.FileName)

	objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
	if err != nil {
		return fmt.Errorf("error uploading file to minio with error: %s", err)
	}

	agentSaveExists := false

	for _, save := range theAgent.Saves {
		if save.FileName == fileIdentity.FileName {
			agentSaveExists = true
			break
		}
	}

	if !agentSaveExists {
		newAgentSave := modelsv2.AgentSave{
			UUID:      fileIdentity.UUID,
			FileName:  fileIdentity.FileName,
			FileUrl:   objectUrl,
			Size:      fileIdentity.Filesize,
			CreatedAt: time.Now(),
		}

		if updateModTime {
			newAgentSave.ModTime = time.Now()
		}

		theAgent.Saves = append(theAgent.Saves, newAgentSave)
	} else {
		for idx := range theAgent.Saves {
			save := &theAgent.Saves[idx]
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
		"saves":     theAgent.Saves,
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

func UploadedAgentBackup(agentAPIKey string, fileIdentity types.StorageFileIdentity) error {
	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAccount := &modelsv2.AccountSchema{}
	filter := bson.M{"agents": bson.M{"$in": bson.A{theAgent.ID}}}

	if err := AccountModel.FindOne(theAccount, filter); err != nil {
		return fmt.Errorf("error finding account with error: %s", err.Error())
	}

	objectPath := fmt.Sprintf("%s/%s/backups/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), fileIdentity.FileName)

	objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
	if err != nil {
		return fmt.Errorf("error uploading file to minio with error: %s", err)
	}

	newAgentBackup := modelsv2.AgentBackup{
		UUID:      fileIdentity.UUID,
		FileName:  fileIdentity.FileName,
		Size:      fileIdentity.Filesize,
		FileUrl:   objectUrl,
		CreatedAt: time.Now(),
	}

	theAgent.Backups = append(theAgent.Backups, newAgentBackup)

	dbUpdate := bson.M{
		"backups":   theAgent.Backups,
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

func UploadedAgentLog(agentAPIKey string, fileIdentity types.StorageFileIdentity) error {
	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	AgentLogModel, err := repositories.GetMongoClient().GetModel("AgentLog")
	if err != nil {
		return err
	}

	theAccount := &modelsv2.AccountSchema{}
	if err := AccountModel.FindOne(theAccount, bson.M{"agents": theAgent.ID}); err != nil {
		return fmt.Errorf("error finding agent account with error: %s", err.Error())
	}

	fileContents, err := utils.ReadLastNBtyesFromFile(fileIdentity.LocalFilePath, 4000)

	if err != nil {
		return fmt.Errorf("error reading log contents with error: %s", err.Error())
	}

	objectPath := fmt.Sprintf("%s/%s/logs/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), fileIdentity.FileName)

	objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
	if err != nil {
		return fmt.Errorf("error uploading file to minio with error: %s", err)
	}

	if err := AgentModel.PopulateField(theAgent, "Logs"); err != nil {
		return fmt.Errorf("error populating agent logs with error: %s", err.Error())
	}

	logType := "FactoryGame"

	if strings.HasPrefix(fileIdentity.FileName, "SSMAgent") {
		logType = "Agent"
	}
	if strings.HasPrefix(fileIdentity.FileName, "Steam") {
		logType = "Steam"
	}

	var theLog *modelsv2.AgentLogSchema
	hasLog := false

	for idx := range theAgent.Logs {
		log := &theAgent.Logs[idx]
		if log.Type == logType {
			hasLog = true
			theLog = log
			break
		}
	}

	if !hasLog {
		theLog := &modelsv2.AgentLogSchema{
			ID:        primitive.NewObjectID(),
			FileName:  fileIdentity.FileName,
			Type:      logType,
			Snippet:   fileContents,
			FileURL:   objectUrl,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := AgentLogModel.Create(theLog); err != nil {
			return fmt.Errorf("error inserting new agent log with error: %s", err.Error())
		}

		theAgent.LogIds = append(theAgent.LogIds, theLog.ID)

		dbUpdate := bson.M{
			"logs":      theAgent.LogIds,
			"updatedAt": time.Now(),
		}

		if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
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

	if err := AgentLogModel.UpdateData(theLog, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}

func GetAgentModConfig(agentAPIKey string) (*modelsv2.AgentModConfig, error) {

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return nil, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return nil, err
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		mod := &theAgent.ModConfig.SelectedMods[idx]
		if err := ModModel.PopulateField(mod, "Mod"); err != nil {
			err = fmt.Errorf("error populating mod with error: %s", err.Error())
			return nil, err
		}
	}

	return &theAgent.ModConfig, nil

}

func UpdateAgentModConfig(agentAPIKey string, newConfig modelsv2.AgentModConfig) error {

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	ModModel, err := repositories.GetMongoClient().GetModel("AgentModConfigSelectedMod")
	if err != nil {
		return err
	}

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		mod := &theAgent.ModConfig.SelectedMods[idx]
		if err := ModModel.PopulateField(mod, "Mod"); err != nil {
			err = fmt.Errorf("error populating mod with error: %s", err.Error())
			return err
		}
	}

	for idx := range theAgent.ModConfig.SelectedMods {
		agentMod := &theAgent.ModConfig.SelectedMods[idx]

		for newIdx := range newConfig.SelectedMods {
			newMod := newConfig.SelectedMods[newIdx]

			if newMod.Mod.ModReference == agentMod.Mod.ModReference {
				agentMod.Installed = newMod.Installed
				agentMod.InstalledVersion = newMod.InstalledVersion
				agentMod.Config = newMod.Config
			}
		}
	}

	dbUpdate := bson.M{
		"modConfig": theAgent.ModConfig,
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

func GetAgentTasksApi(agentAPIKey string) ([]modelsv2.AgentTask, error) {
	tasks := make([]modelsv2.AgentTask, 0)

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return tasks, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	return theAgent.Tasks, nil
}

func UpdateAgentTaskItem(agentAPIKey string, taskId string, newTask modelsv2.AgentTask) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	if err := PurgeAgentTasks(); err != nil {
		return err
	}

	for idx := range theAgent.Tasks {
		task := &theAgent.Tasks[idx]

		if task.ID.Hex() != newTask.ID.Hex() {
			continue
		}

		task.Completed = newTask.Completed
		task.Retries = newTask.Retries
	}

	dbUpdate := bson.M{
		"tasks":     theAgent.Tasks,
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

func GetAgentConfig(agentAPIKey string) (types.API_AgentConfig_ResData, error) {
	var config types.API_AgentConfig_ResData

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return config, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	config.Config = theAgent.Config
	config.ServerConfig = theAgent.ServerConfig

	return config, nil
}

func GetAgentSaves(agentAPIKey string) ([]modelsv2.AgentSave, error) {
	saves := make([]modelsv2.AgentSave, 0)
	agent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return saves, fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	saves = agent.Saves

	return saves, nil

}

func PostAgentSyncSaves(agentAPIKey string, saves []modelsv2.AgentSave) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	newSavesList := make([]modelsv2.AgentSave, 0)

	hasChanged := false

	// Check for new save info
	for updateIdx := range saves {
		updatedSave := &saves[updateIdx]

		found := false

		for agentSaveIdx := range theAgent.Saves {
			agentSave := &theAgent.Saves[agentSaveIdx]

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

		if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
			return err
		}

	}

	return nil
}

func UpdateAgentConfigApi(agentAPIKey string, version string, ip string) error {
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	theAgent.Config.Version = version
	theAgent.Config.IP = ip

	dbUpdate := bson.M{
		"$set": bson.M{
			"config.version": theAgent.Config.Version,
			"config.ip":      theAgent.Config.IP,
			"updatedAt":      time.Now(),
		},
	}

	if err := AgentModel.RawUpdateData(theAgent, dbUpdate); err != nil {
		return err
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}
