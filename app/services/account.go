package services

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/kataras/jwt"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

func LoginAccountUser(email string, password string) (string, error) {

	var theUser models.Users

	if err := mongoose.FindOne(bson.M{"email": email}, &theUser); err != nil {

		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("invalid user details")
		}

		return "", fmt.Errorf("error finding user account with error: %s", err.Error())
	}

	if err := bcrypt.CompareHashAndPassword([]byte(theUser.Password), []byte(password)); err != nil {
		return "", errors.New("invalid user details")
	}

	if _, err := mongoose.DeleteMany(bson.M{"userId": theUser.ID}, "accountsessions"); err != nil {
		return "", fmt.Errorf("error deleting existing account sessions with error: %s", err.Error())
	}

	var theAccount models.Accounts

	if err := mongoose.FindOne(bson.M{"users": theUser.ID}, &theAccount); err != nil {
		return "", fmt.Errorf("error finding account with error: %s", err.Error())
	}

	sessionExpiry := time.Now().AddDate(0, 0, 1)

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
		ID:          primitive.NewObjectID(),
		AccountName: accountName,
		Sessions:    primitive.A{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
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
