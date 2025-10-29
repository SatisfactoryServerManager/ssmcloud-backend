package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	modelsv1 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	accountCleanupJob         *joblock.JobLockTask
	accountWorkflowJob        *joblock.JobLockTask
	inactiveAccountsJob       *joblock.JobLockTask
	deleteInactiveAccountsJob *joblock.JobLockTask
)

func InitAccountService() {

	configData, _ := config.GetConfigData()

	accountCleanupJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"accountCleanupJob", func() {
			if err := CleanupAccountFiles(); err != nil {
				logger.GetErrorLogger().Printf("error running cleanup account files job with error: %s", err.Error())
			}
		},
		30*time.Second,
		10*time.Second,
		false,
	)

	accountWorkflowJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"accountWorkflowJob", func() {
			if err := ProcessWorkflows(); err != nil {
				logger.GetErrorLogger().Printf("error running account workflow job with error: %s", err.Error())
			}
		},
		5*time.Second,
		10*time.Second,
		false,
	)

	inactiveAccountsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"inactiveAccountsJob", func() {
			if err := CheckForInactiveAccounts(); err != nil {
				logger.GetErrorLogger().Printf("error running inactive account job with error: %s", err.Error())
			}
		},
		30*time.Second,
		10*time.Second,
		false,
	)

	deleteInactiveAccountsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"deleteInactiveAccountsJob", func() {
			if err := DeleteInactiveAccounts(); err != nil {
				logger.GetErrorLogger().Printf("error running delete inactive accounts job with error: %s", err.Error())
			}
		},
		1*time.Minute,
		10*time.Second,
		false,
	)

	ctx := context.Background()
	if !configData.Flags.DisablePurgeAccountData {

		if err := accountCleanupJob.Run(ctx); err != nil {
			fmt.Printf("%v\n", err.Error())
		}

		if err := deleteInactiveAccountsJob.Run(ctx); err != nil {
			fmt.Printf("%v\n", err.Error())
		}
	}
	if err := accountWorkflowJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
	if err := inactiveAccountsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
}

func ShutdownAccountService() error {
	accountCleanupJob.UnLock(context.TODO())
	accountWorkflowJob.UnLock(context.TODO())
	inactiveAccountsJob.UnLock(context.TODO())

	logger.GetDebugLogger().Println("Shutdown Account Service")
	return nil
}

func CleanupAccountFiles() error {
	directory := filepath.Join(config.DataDir, "account_data")

	// Get current time
	now := time.Now()

	// Calculate one month ago
	oneMonthAgo := now.AddDate(0, 0, -7)

	// Walk through the directory
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if it's a file and if it's exactly one month old
		if !info.IsDir() && info.ModTime().Before(oneMonthAgo) {
			// Remove the file
			err := os.Remove(path)
			if err != nil {
				return err
			}
			fmt.Printf("Removed file: %s\n", path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	accounts := make([]modelsv1.Accounts, 0)

	if err := mongoose.FindAll(bson.M{}, &accounts); err != nil {
		return err
	}

	for _, account := range accounts {
		agents, err := GetAllAgents(account.ID.Hex())
		if err != nil {
			return err
		}

		for idx := range agents {
			agent := agents[idx]
			objectPath := fmt.Sprintf("%s/%s", account.ID.Hex(), agent.ID.Hex())
			if err := CheckAgentBackups(objectPath, agent); err != nil {
				return err
			}

			if err := CheckAgentSaves(objectPath, agent); err != nil {
				return err
			}
		}
	}

	return nil
}

func CheckAgentSaves(baseObjectPath string, obj *modelsv2.AgentSchema) error {

	if len(obj.Saves) == 0 {
		return nil
	}

	newSavesList := make([]modelsv2.AgentSave, 0)
	for _, save := range obj.Saves {
		objectPath := fmt.Sprintf("%s/saves/%s", baseObjectPath, save.FileName)

		if repositories.HasAgentFile(objectPath) {
			newSavesList = append(newSavesList, save)
		} else {
			fmt.Printf("cant find save file: %s", objectPath)
		}
	}

	if len(obj.Saves) != len(newSavesList) {

		dbUpdate := bson.M{
			"saves":     newSavesList,
			"updatedAt": time.Now(),
		}

		if err := mongoose.UpdateModelData(*obj, dbUpdate); err != nil {
			return err
		}
	}

	return nil
}

func CheckAgentBackups(baseObjectPath string, obj *modelsv2.AgentSchema) error {

	if len(obj.Backups) == 0 {
		return nil
	}

	newBackupsList := make([]modelsv2.AgentBackup, 0)
	for _, backup := range obj.Backups {
		objectPath := fmt.Sprintf("%s/backups/%s", baseObjectPath, backup.FileName)

		if repositories.HasAgentFile(objectPath) {
			newBackupsList = append(newBackupsList, backup)
		} else {
			fmt.Printf("cant find backup file: %s\n", objectPath)
		}
	}

	if len(obj.Backups) != len(newBackupsList) {

		dbUpdate := bson.M{
			"backups":   newBackupsList,
			"updatedAt": time.Now(),
		}

		if err := mongoose.UpdateModelData(*obj, dbUpdate); err != nil {
			return err
		}
	}

	return nil
}

func ProcessWorkflows() error {

	WorkflowModel, err := repositories.GetMongoClient().GetModel("Workflow")
	if err != nil {
		return err
	}

	workflows := make([]modelsv2.WorkflowSchema, 0)

	if err := WorkflowModel.FindAll(&workflows, bson.M{"status": ""}); err != nil {
		return err
	}

	if len(workflows) == 0 {
		return nil
	}

	fmt.Println("Processing Workflows")

	for idx := range workflows {
		workflow := &workflows[idx]

		v2.ValidateStatus(workflow)
		if workflow.Status != "" {
			continue
		}

		if err := v2.ProcessWorkflow(workflow); err != nil {
			return err
		}
	}

	return nil
}

func CheckForInactiveAccounts() error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	allAccounts := make([]modelsv2.AccountSchema, 0)

	if err := AccountModel.FindAll(&allAccounts, bson.M{}); err != nil {
		return err
	}

	inactivityTimeLimit := time.Now().AddDate(0, -2, 0)

	for idx := range allAccounts {
		theAccount := &allAccounts[idx]

		lastActiveTime := time.Time{}

		if err := AccountModel.PopulateField(theAccount, "Agents"); err != nil {
			return err
		}

		for _, agent := range theAccount.Agents {
			if agent.Status.LastCommDate.After(lastActiveTime) {
				lastActiveTime = agent.Status.LastCommDate
			}
		}

		if lastActiveTime.Before(inactivityTimeLimit) && !theAccount.InactivityState.Inactive {
			theAccount.InactivityState.Inactive = true
			theAccount.InactivityState.DateInactive = time.Now()
			theAccount.InactivityState.DeleteDate = time.Now().AddDate(0, 1, 0)

			dbUpdate := bson.M{
				"inactivityState": theAccount.InactivityState,
				"updatedAt":       time.Now(),
			}
			if err := AccountModel.UpdateData(theAccount, dbUpdate); err != nil {
				return err
			}
		} else if lastActiveTime.After(inactivityTimeLimit) && theAccount.InactivityState.Inactive {
			theAccount.InactivityState.Inactive = false
			theAccount.InactivityState.DateInactive = time.Time{}
			theAccount.InactivityState.DeleteDate = time.Time{}

			dbUpdate := bson.M{
				"inactivityState": theAccount.InactivityState,
				"updatedAt":       time.Now(),
			}
			if err := AccountModel.UpdateData(theAccount, dbUpdate); err != nil {
				return err
			}
		}

	}

	return nil
}

func DeleteInactiveAccounts() error {
	defer utils.TrackTime(time.Now(), "DeleteInactiveAccounts")

	var inactiveAccounts []modelsv1.Accounts
	if err := mongoose.FindAll(bson.M{"state.inactive": true, "state.deleteDate": bson.M{"$lt": time.Now()}}, &inactiveAccounts); err != nil {
		return err
	}

	logger.GetDebugLogger().Printf("Found %d inactive accounts ready to delete\n", len(inactiveAccounts))

	for i := range inactiveAccounts {
		account := &inactiveAccounts[i]

		fmt.Printf("deleting account %s\n", account.AccountName)

		fmt.Println("* deleting account storage")
		if err := repositories.DeleteAccountFolder(account.ID.Hex()); err != nil {
			return err
		}

		if err := account.AtomicDelete(); err != nil {
			return err
		}

	}

	return nil
}

func GetAccountSession(sessionIdStr string) (modelsv1.AccountSessions, error) {

	var theSession modelsv1.AccountSessions
	sessionId, err := primitive.ObjectIDFromHex(sessionIdStr)

	if err != nil {
		return theSession, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": sessionId}, &theSession); err != nil {
		return theSession, fmt.Errorf("error finding session with error: %s", err.Error())
	}

	if theSession.Expiry.Before(time.Now()) {
		mongoose.DeleteOne(bson.M{"_id": theSession.ID}, "accountsessions")
		return theSession, fmt.Errorf("error session has expired")
	}

	return theSession, nil
}

func GetAccount(accountIdStr string) (modelsv1.Accounts, error) {
	var theAccount modelsv1.Accounts
	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return theAccount, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return theAccount, fmt.Errorf("error finding session with error: %s", err.Error())
	}

	return theAccount, nil
}

func GetAccountByAgentId(agentIdStr string) (modelsv1.Accounts, error) {
	var theAccount modelsv1.Accounts
	agentId, err := primitive.ObjectIDFromHex(agentIdStr)

	if err != nil {
		return theAccount, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"agents": agentId}, &theAccount); err != nil {
		return theAccount, fmt.Errorf("error finding account with error: %s", err.Error())
	}

	return theAccount, nil
}
