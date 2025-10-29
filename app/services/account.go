package services

import (
	"context"
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	accountCleanupJob         *joblock.JobLockTask
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
	if err := inactiveAccountsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
}

func ShutdownAccountService() error {
	accountCleanupJob.UnLock(context.TODO())
	deleteInactiveAccountsJob.UnLock(context.TODO())
	inactiveAccountsJob.UnLock(context.TODO())

	logger.GetDebugLogger().Println("Shutdown Account Service")
	return nil
}

func CleanupAccountFiles() error {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	accounts := make([]modelsv2.AccountSchema, 0)

	if err := AccountModel.FindAll(&accounts, bson.M{}); err != nil {
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

func CheckAgentSaves(baseObjectPath string, theAgent *modelsv2.AgentSchema) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	if len(theAgent.Saves) == 0 {
		return nil
	}

	newSavesList := make([]modelsv2.AgentSave, 0)
	for _, save := range theAgent.Saves {
		objectPath := fmt.Sprintf("%s/saves/%s", baseObjectPath, save.FileName)

		if repositories.HasAgentFile(objectPath) {
			newSavesList = append(newSavesList, save)
		} else {
			fmt.Printf("cant find save file: %s", objectPath)
		}
	}

	if len(theAgent.Saves) != len(newSavesList) {

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

func CheckAgentBackups(baseObjectPath string, theAgent *modelsv2.AgentSchema) error {

	AgentModel, err := repositories.GetMongoClient().GetModel("Agent")
	if err != nil {
		return err
	}

	if len(theAgent.Backups) == 0 {
		return nil
	}

	newBackupsList := make([]modelsv2.AgentBackup, 0)
	for _, backup := range theAgent.Backups {
		objectPath := fmt.Sprintf("%s/backups/%s", baseObjectPath, backup.FileName)

		if repositories.HasAgentFile(objectPath) {
			newBackupsList = append(newBackupsList, backup)
		} else {
			fmt.Printf("cant find backup file: %s\n", objectPath)
		}
	}

	if len(theAgent.Backups) != len(newBackupsList) {

		dbUpdate := bson.M{
			"backups":   newBackupsList,
			"updatedAt": time.Now(),
		}

		if err := AgentModel.UpdateData(theAgent, dbUpdate); err != nil {
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

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return err
	}

	inactiveAccounts := make([]*modelsv2.AccountSchema, 0)
	if err := AccountModel.FindAll(inactiveAccounts, bson.M{"inactivityState.inactive": true, "inactivityState.deleteDate": bson.M{"$lt": time.Now()}}); err != nil {
		return err
	}

	logger.GetDebugLogger().Printf("Found %d inactive accounts ready to delete\n", len(inactiveAccounts))

	for i := range inactiveAccounts {
		account := inactiveAccounts[i]

		fmt.Printf("deleting account %s\n", account.AccountName)

		fmt.Println("* deleting account storage")
		if err := repositories.DeleteAccountFolder(account.ID.Hex()); err != nil {
			return err
		}

		// TODO: Delete Account
		// if err := account.AtomicDelete(); err != nil {
		// 	return err
		// }

	}

	return nil
}
