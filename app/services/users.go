package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils"
	"github.com/mrhid6/go-mongoose/mongoose"
	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

func GetAllUsers(accountIdStr string) ([]models.Users, error) {

	var theAccount models.Accounts
	emptyUsers := make([]models.Users, 0)

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return emptyUsers, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return emptyUsers, fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateUsers(); err != nil {
		return emptyUsers, fmt.Errorf("error populating account users with error: %s", err.Error())
	}

	return theAccount.UserObjects, nil
}

func GetMyUser(accountIdStr string, userIdStr string) (models.Users, error) {
	var theAccount models.Accounts
	var theUser models.Users

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return theUser, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	userId, err := primitive.ObjectIDFromHex(userIdStr)

	if err != nil {
		return theUser, fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return theUser, fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateUsers(); err != nil {
		return theUser, fmt.Errorf("error populating account users with error: %s", err.Error())
	}

	for _, user := range theAccount.UserObjects {
		if user.ID.Hex() == userId.Hex() {
			theUser = user
			break
		}
	}

	if theUser.ID.IsZero() {
		return theUser, errors.New("error user cant be found")
	}

	return theUser, nil
}

func GetUserByInviteCode(inviteCode string) (models.Users, error) {

	var theUser models.Users

	if err := mongoose.FindOne(bson.M{"inviteCode": inviteCode, "active": false}, &theUser); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return theUser, errors.New("no user with that invite code exists")
		}
	}

	return theUser, nil
}

func GenerateUserTwoFASecret(accountIdStr string, userIdStr string) (string, error) {

	theUser, err := GetMyUser(accountIdStr, userIdStr)

	if err != nil {
		return "", err
	}

	theUser.TwoFAState.TwoFASecret = strings.ToUpper(utils.TwoFASecretGenerator(8))

	dbUpdate := bson.D{{"$set", bson.D{
		{"twoFAState", theUser.TwoFAState},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theUser, dbUpdate); err != nil {
		return "", err
	}

	return theUser.TwoFAState.TwoFASecret, nil
}

func ValidateUserTwoFACode(accountIdStr string, userIdStr string, code string) error {

	theUser, err := GetMyUser(accountIdStr, userIdStr)

	if err != nil {
		return err
	}

	if !totp.Validate(code, theUser.TwoFAState.TwoFASecret) {
		return errors.New("invalid 2fa code")
	}

	if !theUser.TwoFAState.TwoFASetup {
		theUser.TwoFAState.TwoFASetup = true

		dbUpdate := bson.D{{"$set", bson.D{
			{"twoFAState", theUser.TwoFAState},
			{"updatedAt", time.Now()},
		}}}

		if err := mongoose.UpdateDataByID(&theUser, dbUpdate); err != nil {
			return err
		}
	}

	return nil
}

func CreateAccountUser(accountIdStr string, email string) error {

	theAccount, err := GetAccount(accountIdStr)

	if err != nil {
		return err
	}

	allUsers, err := GetAllUsers(accountIdStr)
	if err != nil {
		return err
	}

	for _, user := range allUsers {
		if user.Email == email {
			return errors.New("user already exists")
		}
	}

	inviteCode := strings.ToUpper(utils.RandStringBytes(10))

	newUser := models.Users{
		ID:         primitive.NewObjectID(),
		Email:      email,
		APIKeys:    make([]models.UserAPIKey, 0),
		InviteCode: inviteCode,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	theAccount.Users = append(theAccount.Users, newUser.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"users", theAccount.Users},
		{"updatedAt", time.Now()},
	}}}

	if _, err := mongoose.InsertOne(&newUser); err != nil {
		return fmt.Errorf("error inserting new user with error: %s", err.Error())
	}

	if err := mongoose.UpdateDataByID(&theAccount, dbUpdate); err != nil {
		return err
	}

	theAccount.AddAudit("CREATE_USER", fmt.Sprintf("A new user was created (%s)", newUser.Email))

	return nil
}

func AcceptInviteCode(inviteCode string, password string) error {

	var theUser models.Users

	if err := mongoose.FindOne(bson.M{"inviteCode": inviteCode, "active": false}, &theUser); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("no user with that invite code exists")
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("error creating user with error: %s", err.Error())
	}

	theUser.Password = string(hashedPassword)
	theUser.Active = true
	theUser.InviteCode = ""

	dbUpdate := bson.D{{"$set", bson.D{
		{"password", theUser.Password},
		{"active", theUser.Active},
		{"inviteCode", theUser.InviteCode},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theUser, dbUpdate); err != nil {
		return err
	}

	return nil
}

func CreateUserAPIKey(accountIdStr string, userIdStr string, apiKey string) error {
	theUser, err := GetMyUser(accountIdStr, userIdStr)

	if err != nil {
		return err
	}

	containsKey := false

	for _, key := range theUser.APIKeys {
		if key.Key == apiKey {
			containsKey = true
			break
		}
	}

	if containsKey {
		return fmt.Errorf("error api key already exists on user account")
	}

	newApiKey := models.UserAPIKey{
		Key:      apiKey,
		ShortKey: apiKey[len(apiKey)-6:],
	}

	theUser.APIKeys = append(theUser.APIKeys, newApiKey)

	dbUpdate := bson.D{{"$set", bson.D{
		{"apiKeys", theUser.APIKeys},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theUser, dbUpdate); err != nil {
		return err
	}

	return nil
}

func DeleteUserAPIKey(accountIdStr string, userIdStr string, shortApiKey string) error {
	theUser, err := GetMyUser(accountIdStr, userIdStr)

	if err != nil {
		return err
	}

	newKeyArray := make([]models.UserAPIKey, 0)

	for _, key := range theUser.APIKeys {
		if key.ShortKey != shortApiKey {
			newKeyArray = append(newKeyArray, key)
		}
	}

	theUser.APIKeys = newKeyArray

	dbUpdate := bson.D{{"$set", bson.D{
		{"apiKeys", theUser.APIKeys},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(&theUser, dbUpdate); err != nil {
		return err
	}

	return nil
}
