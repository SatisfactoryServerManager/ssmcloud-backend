package services

import (
	"fmt"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/models"
	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func LoginAccountUser(email string, password string) (*models.AccountSessions, error) {

	var theUser models.Users

	if err := mongoose.FindOne(bson.M{"email": email, "password": password}, &theUser); err != nil {
		return nil, fmt.Errorf("error finding user account with error: %s", err.Error())
	}

	if _, err := mongoose.DeleteMany(bson.M{"userId": theUser.ID}, "accountsessions"); err != nil {
		return nil, fmt.Errorf("error deleting existing account sessions with error: %s", err.Error())
	}

	var theAccount models.Accounts

	if err := mongoose.FindOne(bson.M{"users": theUser.ID}, &theAccount); err != nil {
		return nil, fmt.Errorf("error finding account with error: %s", err.Error())
	}

	sessionExpiry := time.Now().AddDate(0, 0, 1)

	newSession := models.AccountSessions{
		ID:        primitive.NewObjectID(),
		AccountID: theAccount.ID,
		UserID:    theUser.ID,
		Expiry:    sessionExpiry,
	}

	if _, err := mongoose.InsertOne(&newSession); err != nil {
		return nil, fmt.Errorf("error inserting new account session with error: %s", err.Error())
	}

	theAccount.Sessions = append(theAccount.Sessions, newSession.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"sessions", theAccount.Sessions},
	}}}
	if err := mongoose.UpdateDataByID(&theAccount, dbUpdate); err != nil {
		return nil, err
	}

	return &newSession, nil
}
