package models

import (
	"time"

	"github.com/mrhid6/go-mongoose/mongoose"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Accounts struct {
	ID          primitive.ObjectID `json:"_id" bson:"_id"`
	AccountName string             `json:"accountName" bson:"accountName"`

	Sessions       primitive.A       `json:"-" bson:"sessions" mson:"collection=accountsessions"`
	SessionObjects []AccountSessions `json:"sessions" bson:"-"`

	Users       primitive.A `json:"-" bson:"users" mson:"collection=users"`
	UserObjects []Users     `json:"users" bson:"-"`

	Agents       primitive.A `json:"-" bson:"agents" mson:"collection=agents"`
	AgentObjects []Agents    `json:"agents" bson:"-"`

	Audit        primitive.A    `json:"-" bson:"audit" mson:"collection=accountaudit"`
	AuditObjects []AccountAudit `json:"audit" bson:"-"`

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

type AccountAudit struct {
	ID      primitive.ObjectID `json:"_id" bson:"_id"`
	Type    string             `json:"type" bson:"type"`
	Message string             `json:"message" bson:"message"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
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

func (obj *Accounts) PopulateAgents() error {

	if obj.Agents == nil {
		obj.Agents = make(primitive.A, 0)
	}

	err := mongoose.PopulateObjectArray(obj, "Agents", &obj.AgentObjects)

	if err != nil {
		return err
	}

	if obj.AgentObjects == nil {
		obj.AgentObjects = make([]Agents, 0)
	}

	return nil
}

func (obj *Accounts) PopulateAudit() error {

	err := mongoose.PopulateObjectArray(obj, "Audit", &obj.AuditObjects)

	if err != nil {
		return err
	}

	if obj.AuditObjects == nil {
		obj.AuditObjects = make([]AccountAudit, 0)
	}

	return nil
}

func (obj *Accounts) AddAudit(auditType string, message string) error {
	if err := obj.PopulateAudit(); err != nil {
		return err
	}

	newAudit := AccountAudit{
		ID:        primitive.NewObjectID(),
		Type:      auditType,
		Message:   message,
		CreatedAt: time.Now(),
	}

	obj.Audit = append(obj.Audit, newAudit.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"audit", obj.Audit},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		return err
	}

	if _, err := mongoose.InsertOne(&newAudit); err != nil {
		return err
	}

	return nil
}
