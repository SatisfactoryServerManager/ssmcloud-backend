package agent

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/types"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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

	if err := os.Remove(fileIdentity.LocalFilePath); err != nil {
		return fmt.Errorf("error removing temp uploaded log file with error: %s", err.Error())
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
			ID:            bson.NewObjectID(),
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
