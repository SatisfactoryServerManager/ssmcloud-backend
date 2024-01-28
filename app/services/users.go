package services

import (
	"fmt"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/mrhid6/go-mongoose/mongoose"
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
