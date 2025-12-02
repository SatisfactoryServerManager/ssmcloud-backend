package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
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
	uploadPendingLogsJob       *joblock.JobLockTask
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
		1*time.Minute,
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

	uploadPendingLogsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"uploadPendingLogsJob", func() {
			if err := UploadPendingLogs(); err != nil {
				fmt.Println(err)
			}
		},
		30*time.Second, // Run every 30 seconds
		1*time.Minute,  // Lock for 1 minute
		false,
	)

	if err := uploadPendingLogsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
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

	file, err := os.Open(fileIdentity.LocalFilePath)
	if err != nil {
		return err
	}

	buf, err := io.ReadAll(file)

	if err != nil {
		return fmt.Errorf("error reading log contents with error: %s", err.Error())
	}

	fileContents := string(buf)
	file.Close()

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
			ID:            primitive.NewObjectID(),
			FileName:      fileIdentity.FileName,
			Type:          logType,
			LogLines:      strings.Split(fileContents, "\n"),
			PendingUpload: true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
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

	theLog.LogLines = strings.Split(fileContents, "\n")
	theLog.FileName = fileIdentity.FileName

	dbUpdate := bson.M{
		"lines":         theLog.LogLines,
		"fileName":      theLog.FileName,
		"pendingUpload": true,
		"updatedAt":     time.Now(),
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

func UpdateAgentModConfig(agentAPIKey string, newConfig *modelsv2.AgentModConfig) error {

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

func MarkAgentTaskCompleted(agentAPIKey string, taskId string) error {
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

		if task.ID.Hex() != taskId {
			continue
		}

		task.Completed = true
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

func MarkAgentTaskFailed(agentAPIKey string, taskId string) error {
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

		if task.ID.Hex() != taskId {
			continue
		}

		task.Retries = task.Retries + 1
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

func UploadPendingLogs() error {
	AgentLogModel, err := repositories.GetMongoClient().GetModel("AgentLog")
	if err != nil {
		return fmt.Errorf("failed to get AgentLog model: %s", err.Error())
	}

	// Find all logs with pendingUpload = true
	pendingLogs := make([]modelsv2.AgentLogSchema, 0)
	if err := AgentLogModel.FindAll(&pendingLogs, bson.M{"pendingUpload": true}); err != nil {
		return fmt.Errorf("failed to find pending logs: %s", err.Error())
	}

	for idx := range pendingLogs {
		theLog := &pendingLogs[idx]

		// Get the agent that owns this log
		AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
		if err != nil {
			continue
		}

		theAgent := &modelsv2.AgentSchema{}
		if err := AgentModel.FindOne(theAgent, bson.M{"logs": theLog.ID}); err != nil {
			logger.GetErrorLogger().Printf("Failed to find agent for log %s: %s", theLog.ID.Hex(), err.Error())
			continue
		}

		// Get the account that owns this agent
		AccountModel, err := repositories.GetMongoClient().GetModel("Account")
		if err != nil {
			continue
		}

		theAccount := &modelsv2.AccountSchema{}
		if err := AccountModel.FindOne(theAccount, bson.M{"agents": theAgent.ID}); err != nil {
			logger.GetErrorLogger().Printf("Failed to find account for agent %s: %s", theAgent.ID.Hex(), err.Error())
			continue
		}

		// Create temporary file with log content
		tempFile, err := os.CreateTemp("", "ssm-log-*")
		if err != nil {
			logger.GetErrorLogger().Printf("Failed to create temp file: %s", err.Error())
			continue
		}
		defer os.Remove(tempFile.Name())

		// Write log lines to file
		content := strings.Join(theLog.LogLines, "\n")
		if _, err := tempFile.WriteString(content); err != nil {
			logger.GetErrorLogger().Printf("Failed to write to temp file: %s", err.Error())
			continue
		}
		tempFile.Close()

		// Prepare upload
		fileIdentity := types.StorageFileIdentity{
			UUID:          primitive.NewObjectID().Hex(),
			FileName:      theLog.FileName,
			LocalFilePath: tempFile.Name(),
		}

		// Upload to Minio
		objectPath := fmt.Sprintf("%s/%s/logs/%s", theAccount.ID.Hex(), theAgent.ID.Hex(), theLog.FileName)
		objectUrl, err := repositories.UploadAgentFile(fileIdentity, objectPath)
		if err != nil {
			logger.GetErrorLogger().Printf("Failed to upload log %s: %s", theLog.ID.Hex(), err.Error())
			continue
		}

		// Update log with new URL and clear pending flag
		dbUpdate := bson.M{
			"fileUrl":       objectUrl,
			"pendingUpload": false,
			"updatedAt":     time.Now(),
		}

		if err := AgentLogModel.UpdateData(theLog, dbUpdate); err != nil {
			logger.GetErrorLogger().Printf("Failed to update log %s: %s", theLog.ID.Hex(), err.Error())
			continue
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

func AddAgentLogLine(agentAPIKey string, source string, line string, inital bool) error {
	theAgent, err := GetAgentByAPIKey(agentAPIKey)
	if err != nil {
		return fmt.Errorf("error finding agent with error: %s", err.Error())
	}

	AgentLogModel, err := repositories.GetMongoClient().GetModel("AgentLog")
	if err != nil {
		return fmt.Errorf("failed to get AgentLog model: %s", err.Error())
	}
	// Ensure the agent's Logs are populated so we can find/update the correct log
	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return fmt.Errorf("failed to get Agent model: %s", err.Error())
	}

	if err := AgentModel.PopulateField(theAgent, "Logs"); err != nil {
		return fmt.Errorf("failed to populate agent logs: %s", err.Error())
	}

	var theLog *modelsv2.AgentLogSchema

	for idx := range theAgent.Logs {
		log := &theAgent.Logs[idx]
		if log.Type == source {
			theLog = log
			break
		}
	}

	// If no log exists for this source, return due to the full file being uploaded separately
	if theLog == nil {
		return nil
	}

	if inital {
		theLog.LogLines = make([]string, 0)
	}

	// Append the new line to the existing log's LogLines
	theLog.LogLines = append(theLog.LogLines, line)
	theLog.UpdatedAt = time.Now()

	// Update the log entry with new content and mark for upload
	dbUpdate := bson.M{
		"lines":         theLog.LogLines,
		"updatedAt":     theLog.UpdatedAt,
		"pendingUpload": true,
	}

	if err := AgentLogModel.UpdateData(theLog, dbUpdate); err != nil {
		return fmt.Errorf("failed to update agent log: %s", err.Error())
	}

	if err := UpdateAgentLastComm(agentAPIKey); err != nil {
		return err
	}

	return nil
}
