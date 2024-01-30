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

func GenerateUserTwoFASecret(accountIdStr string, userIdStr string) (string, error) {

	theUser, err := GetMyUser(accountIdStr, userIdStr)

	if err != nil {
		return "", err
	}

	theUser.TwoFAState.TwoFASecret = strings.ToUpper(utils.RandStringBytes(8))

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
