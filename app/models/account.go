package models

import (
	"time"

	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Accounts struct {
	ID          primitive.ObjectID `json:"_id" bson:"_id"`
	AccountName string             `json:"accountName" bson:"accountName"`

	Sessions       primitive.A       `json:"-" bson:"sessions" mson:"collection=accountsessions"`
	SessionObjects []AccountSessions `json:"sessions" bson:"-"`

	Users       primitive.A `json:"-" bson:"users" mson:"collection=users"`
	UserObjects []Users     `json:"users" bson:"-"`

	State AccountState `json:"state" bson:"state"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type AccountSessions struct {
	ID        primitive.ObjectID `json:"_id" bson:"_id"`
	AccountID primitive.ObjectID `json:"accountId" bson:"accountId"`
	UserID    primitive.ObjectID `json:"userId" bson:"userId"`
	Expiry    time.Time          `json:"expiry" bson:"expiry"`
}

type AccountState struct {
	Inactive       bool      `json:"inactive" bson:"inactive"`
	InactivityDate time.Time `json:"inactivityDate" bson:"inactivityDate"`
}

func (obj *Accounts) PopulateSessions() error {

	err := mongoose.PopulateObjectArray(obj, "Sessions", &obj.SessionObjects)

	if err != nil {
		return err
	}

	if obj.SessionObjects == nil {
		obj.SessionObjects = make([]AccountSessions, 0)
	}

	return nil
}

func (obj *Accounts) PopulateUsers() error {

	err := mongoose.PopulateObjectArray(obj, "Users", &obj.UserObjects)

	if err != nil {
		return err
	}

	if obj.UserObjects == nil {
		obj.UserObjects = make([]Users, 0)
	}

	return nil
}
