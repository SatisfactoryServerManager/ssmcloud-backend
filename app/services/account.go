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
				fmt.Println(err)
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
				fmt.Println(err)
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
				fmt.Println(err)
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
				fmt.Println(err)
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
			agent := &agents[idx]
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

func CheckAgentSaves(baseObjectPath string, obj *modelsv1.Agents) error {

	if len(obj.Saves) == 0 {
		return nil
	}

	newSavesList := make([]modelsv1.AgentSave, 0)
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

func CheckAgentBackups(baseObjectPath string, obj *modelsv1.Agents) error {

	if len(obj.Backups) == 0 {
		return nil
	}

	newBackupsList := make([]modelsv1.AgentBackup, 0)
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
	allAccounts := make([]modelsv1.Accounts, 0)

	if err := mongoose.FindAll(bson.M{}, &allAccounts); err != nil {
		return err
	}

	inactivityTimeLimit := time.Now().AddDate(0, -2, 0)

	for idx := range allAccounts {
		account := &allAccounts[idx]

		lastActiveTime := time.Time{}

		if err := account.PopulateAgents(); err != nil {
			return err
		}

		if err := account.PopulateUsers(); err != nil {
			return err
		}

		for _, agent := range account.AgentObjects {
			if agent.Status.LastCommDate.After(lastActiveTime) {
				lastActiveTime = agent.Status.LastCommDate
			}
		}

		for _, user := range account.UserObjects {
			if user.LastActive.After(lastActiveTime) {
				lastActiveTime = user.LastActive
			}
		}

		if lastActiveTime.Before(inactivityTimeLimit) && !account.State.Inactive {
			account.State.Inactive = true
			account.State.InactivityDate = time.Now()
			account.State.DeleteDate = time.Now().AddDate(0, 1, 0)

			dbUpdate := bson.D{{"$set", bson.D{
				{"state", account.State},
				{"updatedAt", time.Now()},
			}}}
			if err := mongoose.UpdateDataByID(*account, dbUpdate); err != nil {
				return err
			}
		} else if lastActiveTime.After(inactivityTimeLimit) && account.State.Inactive {
			account.State.Inactive = false
			account.State.InactivityDate = time.Time{}
			account.State.DeleteDate = time.Time{}

			dbUpdate := bson.D{{"$set", bson.D{
				{"state", account.State},
				{"updatedAt", time.Now()},
			}}}
			if err := mongoose.UpdateDataByID(*account, dbUpdate); err != nil {
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
