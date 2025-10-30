package v2

import (
	"fmt"
	"html/template"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	v1 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetMyUser(id primitive.ObjectID, externalId string, email string, username string) (*models.UserSchema, error) {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	theUser := &models.UserSchema{}
	filter := bson.M{"$or": bson.A{bson.M{"eid": externalId}, bson.M{"_id": id}, bson.M{"email": email}}}
	if err := UserModel.FindOne(theUser, filter); err != nil {
		return nil, fmt.Errorf("error getting user with error: %s", err.Error())
	}

	if externalId != "" && externalId != theUser.ExternalID {
		theUser.ExternalID = externalId

		update := bson.M{"eid": theUser.ExternalID}
		if err := UserModel.UpdateData(theUser, update); err != nil {
			return nil, err
		}
	}

	if username != "" && username != theUser.Username {
		theUser.Username = username

		update := bson.M{"username": theUser.Username}
		if err := UserModel.UpdateData(theUser, update); err != nil {
			return nil, err
		}
	}

	if len(theUser.LinkedAccountIds) == 0 && theUser.ActiveAccountId.IsZero() {

		accounts := make([]v1.Accounts, 0)
		filter := bson.M{"users": bson.M{"$in": bson.A{theUser.ID}}}
		mongoose.FindAll(filter, &accounts)

		if len(accounts) > 0 {
			for _, account := range accounts {
				theUser.LinkedAccountIds = append(theUser.LinkedAccountIds, account.ID)
			}
			theUser.ActiveAccountId = theUser.LinkedAccountIds[0].(primitive.ObjectID)

			update := bson.M{
				"linkedAccounts": theUser.LinkedAccountIds,
				"activeAccount":  theUser.ActiveAccountId,
			}
			if err := UserModel.UpdateData(theUser, update); err != nil {
				return nil, err
			}
		}
	}

	theUser.ProfileImageURL = template.URL(theUser.ProfileImageStr)

	return theUser, nil
}

func UpdateUserProfilePicture(theUser *models.UserSchema, avatarUrl string) error {
	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	if avatarUrl == "" {
		return nil
	}

	if theUser.ProfileImageURL != "" {
		return nil
	}

	if err := UserModel.UpdateData(theUser, bson.M{"profileImageUrl": avatarUrl}); err != nil {
		return fmt.Errorf("error updating user avatar with error: %s", err.Error())
	}

	return nil
}
