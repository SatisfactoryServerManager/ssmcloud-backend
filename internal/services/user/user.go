package user

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	v1 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v1"
	models "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetUser(id bson.ObjectID, externalId string, email string, username string) (*models.UserSchema, error) {

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
			theUser.ActiveAccountId = theUser.LinkedAccountIds[0].(bson.ObjectID)

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

func CreateUser(eid string, email string, username string) (*models.UserSchema, error) {

	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return nil, err
	}

	NewUser := &models.UserSchema{
		ID:         bson.NewObjectID(),
		ExternalID: eid,
		Email:      email,
		Username:   username,
		APIKeys:    make([]models.UserAPIKey, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := UserModel.Create(NewUser); err != nil {
		return nil, err
	}

	return NewUser, nil
}

func UpdateUserProfilePicture(theUser *models.UserSchema, avatarUrl string) error {
	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	if avatarUrl == "" {
		return nil
	}

	// The identity provider owns the avatar, so follow it whenever it changes.
	if theUser.ProfileImageStr == avatarUrl {
		return nil
	}

	theUser.ProfileImageStr = avatarUrl
	theUser.ProfileImageURL = template.URL(avatarUrl)

	if err := UserModel.UpdateData(theUser, bson.M{"profileImageUrl": avatarUrl}); err != nil {
		return fmt.Errorf("error updating user avatar with error: %s", err.Error())
	}

	return nil
}

// CreateUserAPIKey generates a new personal API key for the user. The full key
// is returned once here and never leaves the database again - the UI only ever
// shows the short key after this point.
func CreateUserAPIKey(theUser *models.UserSchema) (models.UserAPIKey, error) {
	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return models.UserAPIKey{}, err
	}

	secret := make([]byte, 24)
	if _, err := cryptorand.Read(secret); err != nil {
		return models.UserAPIKey{}, fmt.Errorf("error generating api key with error: %s", err.Error())
	}

	encoded := hex.EncodeToString(secret)
	newKey := models.UserAPIKey{
		Key:      "API-" + encoded,
		ShortKey: encoded[len(encoded)-4:],
	}

	theUser.APIKeys = append(theUser.APIKeys, newKey)

	if err := UserModel.UpdateData(theUser, bson.M{"apiKeys": theUser.APIKeys}); err != nil {
		return models.UserAPIKey{}, fmt.Errorf("error saving api key with error: %s", err.Error())
	}

	return newKey, nil
}

func DeleteUserAPIKey(theUser *models.UserSchema, shortKey string) error {
	UserModel, err := repositories.GetMongoClient().GetModel("User")
	if err != nil {
		return err
	}

	newKeys := make([]models.UserAPIKey, 0)
	found := false
	for _, key := range theUser.APIKeys {
		if key.ShortKey == shortKey {
			found = true
			continue
		}
		newKeys = append(newKeys, key)
	}

	if !found {
		return fmt.Errorf("error api key (%s) doesnt exist on user", shortKey)
	}

	theUser.APIKeys = newKeys

	if err := UserModel.UpdateData(theUser, bson.M{"apiKeys": theUser.APIKeys}); err != nil {
		return fmt.Errorf("error deleting api key with error: %s", err.Error())
	}

	return nil
}
