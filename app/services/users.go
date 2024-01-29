package services

import (
	"fmt"

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

	return theUser, nil
}

func GenerateUserTwoFASecret(accountIdStr string, userIdStr string) (string, error){
	var theAccount models.Accounts
	var theUser models.Users

	accountId, err := primitive.ObjectIDFromHex(accountIdStr)

	if err != nil {
		return "", fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	userId, err := primitive.ObjectIDFromHex(userIdStr)

	if err != nil {
		return "", fmt.Errorf("error converting query string to object id with error: %s", err.Error())
	}

	if err := mongoose.FindOne(bson.M{"_id": accountId}, &theAccount); err != nil {
		return "", fmt.Errorf("error finding account from session with error: %s", err.Error())
	}

	if err := theAccount.PopulateUsers(); err != nil {
		return "", fmt.Errorf("error populating account users with error: %s", err.Error())
	}

	for idx := range theAccount.UserObjects {
		user:= theAccount.UserObjects[idx];
		if user.ID.Hex() != userId.Hex() {
			continue;
		}

		user.TwoFAState.TwoFASecret = utils.
	}


}