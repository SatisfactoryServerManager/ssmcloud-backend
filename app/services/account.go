package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models"
	modelsv1 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	modelsv2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gtuk/discordwebhook"
	"github.com/kataras/jwt"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	accountCleanupJob           *joblock.JobLockTask
	accountIntegrationEventsJob *joblock.JobLockTask
	accountWorkflowJob          *joblock.JobLockTask
	inactiveAccountsJob         *joblock.JobLockTask
	deleteInactiveAccountsJob   *joblock.JobLockTask
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

	accountIntegrationEventsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"accountIntegrationEventsJob", func() {
			if err := ProcessAccountIntegrationEvents(); err != nil {
				fmt.Println(err)
			}
		},
		5*time.Second,
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
	if err := accountIntegrationEventsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
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
	accountIntegrationEventsJob.UnLock(context.TODO())
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

func ProcessAccountIntegrationEvents() error {

	accounts := make([]modelsv1.Accounts, 0)

	if err := mongoose.FindAll(bson.M{}, &accounts); err != nil {
		return err
	}

	for idx := range accounts {
		account := &accounts[idx]

		for idx := range account.IntegrationObjects {
			integration := &account.IntegrationObjects[idx]
			if err := ProcessIntegrationEvents(integration); err != nil {
				logger.GetErrorLogger().Printf("failed to process integration events for account %s with error %s\n", account.AccountName, err.Error())
			}
		}
	}

	return nil
}

func ProcessIntegrationEvents(obj *modelsv1.AccountIntegrations) error {

	for idx := range obj.Events {
		event := &obj.Events[idx]

		if event.Completed {
			continue
		}

		if event.Failed {
			continue
		}

		if obj.Type == modelsv1.IntegrationWebhook {
			resp, err := ProcessWebhookEvent(obj.Url, event)
			if err != nil {
				event.Status = "failed"
				event.Retries += 1
				event.ResponseData = resp

				if event.Retries >= 10 {
					event.Failed = true
					event.Status = "failed - max retries"
				}
			} else {
				event.Completed = true
				event.Status = "delivered"
				event.ResponseData = resp
			}
		} else if obj.Type == modelsv1.IntegrationDiscord {
			if err := ProcessDiscordEvent(obj.Url, event); err != nil {
				event.Status = "failed"

				event.Retries += 1
				event.ResponseData = fmt.Sprintf(`{"success":false,"error":%s`, err.Error())

				if event.Retries >= 10 {
					event.Failed = true
					event.Status = "failed - max retries"
				}
			} else {
				event.Completed = true
				event.Status = "delivered"
				event.ResponseData = `{"success":true}`
			}
		}
	}

	dbUpdate := bson.M{
		"events":    obj.Events,
		"updatedAt": time.Now(),
	}

	if err := mongoose.UpdateModelData(*obj, dbUpdate); err != nil {
		return err
	}

	return nil
}

func ProcessWebhookEvent(url string, event *modelsv1.AccountIntegrationEvent) (string, error) {

	// Marshal the data into JSON
	jsonBytes, err := json.Marshal(event.Data)
	if err != nil {
		return "", err
	}

	// Prepare the webhook request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the webhook request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			logger.GetErrorLogger().Printf("error closing response body: %s", err)
		}
	}(resp.Body)

	// Determine the status based on the response code
	status := "failed"
	if resp.StatusCode == http.StatusOK {
		status = "delivered"
	}

	if status == "failed" {
		return bodyString, errors.New(status)
	}

	return bodyString, nil
}

func ProcessDiscordEvent(url string, event *modelsv1.AccountIntegrationEvent) error {

	EventNameStr := "SSM Event"
	fields := make([]discordwebhook.Field, 0)
	imageUrl := "https://ssmcloud.hostxtra.co.uk/public/images/ssm_logo256.png"

	eventTimeStr := ""

	switch event.Type {
	case modelsv1.IntegrationEventTypeAgentOnline:
		EventNameStr = "Agent Online"
		data := models.EventDataAgentOnline{}
		MarshalToEventData(event.Data, &data)

		fieldName := "AgentName"
		fieldValue := data.AgentName
		inline := true

		fields = append(fields, discordwebhook.Field{Name: &fieldName, Value: &fieldValue, Inline: &inline})
		eventTimeStr = data.EventData.EventTime.Format("2006-01-02 15:04:05")
	case modelsv1.IntegrationEventTypeAgentOffline:
		EventNameStr = "Agent Offline"
		data := models.EventDataAgentOffline{}
		MarshalToEventData(event.Data, &data)

		fieldName := "AgentName"
		fieldValue := data.AgentName
		inline := true

		fields = append(fields, discordwebhook.Field{Name: &fieldName, Value: &fieldValue, Inline: &inline})
		eventTimeStr = data.EventData.EventTime.Format("2006-01-02 15:04:05")
	}

	footer := discordwebhook.Footer{
		Text: &eventTimeStr,
	}

	embed := discordwebhook.Embed{
		Title:  &EventNameStr,
		Fields: &fields,
		Footer: &footer,
	}

	username := "SSM Cloud"
	message := discordwebhook.Message{
		Username:  &username,
		Embeds:    &[]discordwebhook.Embed{embed},
		AvatarUrl: &imageUrl,
	}

	if err := discordwebhook.SendMessage(url, message); err != nil {
		return err
	}

	return nil
}

func MarshalToEventData(data interface{}, output interface{}) {
	bodyBytes, _ := json.Marshal(data)
	json.Unmarshal(bodyBytes, output)
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

func LoginAccountUser(email string, password string) (string, error) {

	var theUser modelsv1.Users

	if err := mongoose.FindOne(bson.M{"email": email}, &theUser); err != nil {

		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("invalid user details")
		}

		return "", fmt.Errorf("error finding user account with error: %s", err.Error())
	}

	var theAccount modelsv1.Accounts

	if err := mongoose.FindOne(bson.M{"users": theUser.ID}, &theAccount); err != nil {
		return "", fmt.Errorf("error finding account with error: %s", err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(theUser.Password), []byte(password)); err != nil {
		theAccount.AddAudit("LOGIN_FAILURE", fmt.Sprintf("Login failed using %s", theUser.Email))
		return "", errors.New("invalid user details")
	}

	if theAccount.State.Inactive {
		return "", fmt.Errorf("error account is marked as inactive due to inactivity")
	}

	var existingSession modelsv1.AccountSessions
	mongoose.FindOne(bson.M{"userId": theUser.ID}, &existingSession)

	sessionExpiry := time.Now().AddDate(0, 0, 1)

	if !existingSession.ID.IsZero() {

		if existingSession.Expiry.Before(time.Now()) {

			if _, err := mongoose.DeleteOne(bson.M{"_id": existingSession.ID}, "accountsessions"); err != nil {
				return "", err
			}

			if err := theAccount.PopulateSessions(); err != nil {
				return "", err
			}

			newSessionIds := make(primitive.A, 0)
			for _, session := range theAccount.SessionObjects {
				if session.ID.IsZero() {
					continue
				}

				if session.ID.Hex() != existingSession.ID.Hex() {
					newSessionIds = append(newSessionIds, session.ID)
				}
			}

			theAccount.Sessions = newSessionIds

		} else {

			dbUpdate := bson.D{{"$set", bson.D{
				{"expiry", sessionExpiry},
			}}}
			if err := mongoose.UpdateDataByID(&existingSession, dbUpdate); err != nil {
				return "", err
			}

			myClaims := app.Middleware_Session_JWT{
				SessionID: existingSession.ID.Hex(),
				AccountID: existingSession.AccountID.Hex(),
				UserID:    existingSession.UserID.Hex(),
			}

			token, err := jwt.Sign(jwt.HS256, []byte(os.Getenv("JWT_KEY")), myClaims, jwt.MaxAge(24*time.Hour))
			if err != nil {
				return "", err
			}

			theAccount.AddAudit("LOGIN_SUCCESS", fmt.Sprintf("Login successful using %s", theUser.Email))

			return string(token), nil
		}

	}

	newSession := modelsv1.AccountSessions{
		ID:        primitive.NewObjectID(),
		AccountID: theAccount.ID,
		UserID:    theUser.ID,
		Expiry:    sessionExpiry,
	}

	if _, err := mongoose.InsertOne(&newSession); err != nil {
		return "", fmt.Errorf("error inserting new account session with error: %s", err.Error())
	}

	theAccount.Sessions = append(theAccount.Sessions, newSession.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"sessions", theAccount.Sessions},
		{"updatedAt", time.Now()},
	}}}
	if err := mongoose.UpdateDataByID(&theAccount, dbUpdate); err != nil {
		return "", err
	}

	dbUpdate = bson.D{{"$set", bson.D{
		{"updatedAt", time.Now()},
		{"lastActive", time.Now()},
	}}}
	if err := mongoose.UpdateDataByID(&theUser, dbUpdate); err != nil {
		return "", err
	}

	myClaims := app.Middleware_Session_JWT{
		SessionID: newSession.ID.Hex(),
		AccountID: newSession.AccountID.Hex(),
		UserID:    newSession.UserID.Hex(),
	}

	token, err := jwt.Sign(jwt.HS256, []byte(os.Getenv("JWT_KEY")), myClaims, jwt.MaxAge(24*time.Hour))
	if err != nil {
		return "", err
	}

	theAccount.AddAudit("LOGIN_SUCCESS", fmt.Sprintf("Login successful using %s", theUser.Email))

	return string(token), nil
}

func AccountSignup(accountName string, email string, password string) error {

	if accountName == "" {
		return errors.New("error account name is required")
	}

	if email == "" {
		return errors.New("error email is required")
	}

	if password == "" {
		return errors.New("error password is required")
	}

	if len(accountName) < 4 {
		return errors.New("error account name must be more that 4 characters")
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("error email was invalid")
	}

	var existingAccount modelsv1.Accounts
	var existingUser modelsv1.Users

	mongoose.FindOne(bson.M{"accountName": accountName}, &existingAccount)
	mongoose.FindOne(bson.M{"email": email}, &existingUser)

	if !existingAccount.ID.IsZero() {
		return fmt.Errorf("account with name %s already exists", accountName)
	}

	if !existingUser.ID.IsZero() {
		return fmt.Errorf("email %s already in use by another user", email)
	}

	newAccount := modelsv1.Accounts{
		ID:           primitive.NewObjectID(),
		AccountName:  accountName,
		Sessions:     primitive.A{},
		Users:        primitive.A{},
		Integrations: primitive.A{},
		Audit:        primitive.A{},
		Agents:       primitive.A{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("error creating user with error: %s", err.Error())
	}

	newUser := modelsv1.Users{
		ID:             primitive.NewObjectID(),
		Email:          email,
		Password:       string(hashedPassword),
		IsAccountAdmin: true,
		APIKeys:        make([]modelsv1.UserAPIKey, 0),
		Active:         true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	newAccount.Users = append(newAccount.Users, newUser.ID)

	if _, err := mongoose.InsertOne(&newUser); err != nil {
		return fmt.Errorf("error inserting new user with error: %s", err.Error())
	}

	if _, err := mongoose.InsertOne(&newAccount); err != nil {
		return fmt.Errorf("error inserting new account with error: %s", err.Error())
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
