package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
			UUID:          bson.NewObjectID().Hex(),
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

	// No log document exists yet for this source. The agent streams log lines
	// without a separate full-file upload, so create the document on first
	// line rather than dropping the data.
	if theLog == nil {
		logger.GetDebugLogger().Printf("creating new AgentLog for agent %s source %q", theAgent.ID.Hex(), source)

		theLog = &modelsv2.AgentLogSchema{
			ID:            bson.NewObjectID(),
			Type:          source,
			LogLines:      make([]string, 0),
			PendingUpload: true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		if err := AgentLogModel.Create(theLog); err != nil {
			return fmt.Errorf("error creating agent log: %s", err.Error())
		}

		theAgent.LogIds = append(theAgent.LogIds, theLog.ID)
		if err := AgentModel.UpdateData(theAgent, bson.M{"logs": theAgent.LogIds, "updatedAt": time.Now()}); err != nil {
			return fmt.Errorf("error attaching agent log: %s", err.Error())
		}
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
