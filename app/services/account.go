package services

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/joblock"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/config"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	"github.com/kataras/jwt"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	accountCleanupJob           joblock.JobLockTask
	accountIntegrationEventsJob joblock.JobLockTask
	accountWorkflowJob          joblock.JobLockTask
)

func InitAccountService() {

	accountCleanupJob = joblock.JobLockTask{
		Name:     "accountCleanupJob",
		Interval: 30 * time.Second,
		Timeout:  10 * time.Second,
		Arg: func() {
			if err := CleanupAccountFiles(); err != nil {
				fmt.Println(err)
			}
		},
	}

	accountIntegrationEventsJob = joblock.JobLockTask{
		Name:     "accountIntegrationEventsJob",
		Interval: 5 * time.Second,
		Timeout:  10 * time.Second,
		Arg: func() {
			if err := ProcessAccountIntegrationEvents(); err != nil {
				fmt.Println(err)
			}
		},
	}

	accountWorkflowJob = joblock.JobLockTask{
		Name:     "accountWorkflowJob",
		Interval: 10 * time.Second,
		Timeout:  10 * time.Second,
		Arg: func() {
			if err := ProcessAccountWorkflows(); err != nil {
				fmt.Println(err)
			}
		},
	}

	ctx := context.Background()
	if err := accountCleanupJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
	if err := accountIntegrationEventsJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
	if err := accountWorkflowJob.Run(ctx); err != nil {
		fmt.Printf("%v\n", err.Error())
	}
}

func ShutdownAccountService() error {
	accountCleanupJob.UnLock(context.TODO())
	logger.GetDebugLogger().Println("Shutdown Account Service")
	return nil
}

func CleanupAccountFiles() error {
	directory := filepath.Join(config.DataDir, "account_data")

	// Get current time
	now := time.Now()

	// Calculate one month ago
	oneMonthAgo := now.AddDate(0, -1, 0)

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

	accounts := make([]models.Accounts, 0)

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
			agentDataPath := filepath.Join(directory, account.ID.Hex(), agent.ID.Hex())
			if err := agent.CheckBackups(agentDataPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func ProcessAccountIntegrationEvents() error {

	accounts := make([]models.Accounts, 0)

	if err := mongoose.FindAll(bson.M{}, &accounts); err != nil {
		return err
	}

	for idx := range accounts {
		account := &accounts[idx]

		if err := account.ProcessIntegrationEvents(); err != nil {
			logger.GetErrorLogger().Printf("failed to process integration events for account %s with error %s\n", account.AccountName, err.Error())
		}
	}

	return nil
}

func ProcessAccountWorkflows() error {

	workflows := make([]models.Workflows, 0)

	if err := mongoose.FindAll(bson.M{"status": ""}, &workflows); err != nil {
		return err
	}

	if len(workflows) == 0 {
		return nil
	}

	fmt.Println("Process Account Workflows")

	for idx := range workflows {
		workflow := &workflows[idx]
		workflow.ValidateStatus()

		if workflow.Status != "" {
			continue
		}

		workflow.ProcessCurrentAction()
	}

	return nil
}

func LoginAccountUser(email string, password string) (string, error) {

	var theUser models.Users

	if err := mongoose.FindOne(bson.M{"email": email}, &theUser); err != nil {

		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("invalid user details")
		}

		return "", fmt.Errorf("error finding user account with error: %s", err.Error())
	}

	var theAccount models.Accounts

	if err := mongoose.FindOne(bson.M{"users": theUser.ID}, &theAccount); err != nil {
		return "", fmt.Errorf("error finding account with error: %s", err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(theUser.Password), []byte(password)); err != nil {
		theAccount.AddAudit("LOGIN_FAILURE", fmt.Sprintf("Login failed using %s", theUser.Email))
		return "", errors.New("invalid user details")
	}

	var existingSession models.AccountSessions
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

	newSession := models.AccountSessions{
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

	var existingAccount models.Accounts
	var existingUser models.Users

	mongoose.FindOne(bson.M{"accountName": accountName}, &existingAccount)
	mongoose.FindOne(bson.M{"email": email}, &existingUser)

	if !existingAccount.ID.IsZero() {
		return fmt.Errorf("account with name %s already exists", accountName)
	}

	if !existingUser.ID.IsZero() {
		return fmt.Errorf("email %s already in use by another user", email)
	}

	newAccount := models.Accounts{
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

	newUser := models.Users{
		ID:             primitive.NewObjectID(),
		Email:          email,
		Password:       string(hashedPassword),
		IsAccountAdmin: true,
		APIKeys:        make([]models.UserAPIKey, 0),
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

func GetAccountSession(sessionIdStr string) (models.AccountSessions, error) {

	var theSession models.AccountSessions
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

func GetAccount(accountIdStr string) (models.Accounts, error) {
	var theAccount models.Accounts
	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return theAccount, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return theAccount, fmt.Errorf("error finding session with error: %s", err.Error())
	}

	return theAccount, nil
}

func GetAccountByAgentId(agentIdStr string) (models.Accounts, error) {
	var theAccount models.Accounts
	agentId, err := primitive.ObjectIDFromHex(agentIdStr)

	if err != nil {
		return theAccount, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"agents": agentId}, &theAccount); err != nil {
		return theAccount, fmt.Errorf("error finding account with error: %s", err.Error())
	}

	return theAccount, nil
}
