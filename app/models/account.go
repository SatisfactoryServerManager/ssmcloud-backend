package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	"github.com/gtuk/discordwebhook"
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

	Integrations       primitive.A           `json:"-" bson:"integrations" mson:"collection=accountintegrations"`
	IntegrationObjects []AccountIntegrations `json:"integrations" bson:"-"`

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
	DeleteDate     time.Time `json:"deleteDate" bson:"deleteDate"`
}

type AccountAudit struct {
	ID      primitive.ObjectID `json:"_id" bson:"_id"`
	Type    string             `json:"type" bson:"type"`
	Message string             `json:"message" bson:"message"`

	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
}

type IntegrationType int64

const (
	IntegrationWebhook IntegrationType = 0
	IntegrationDiscord IntegrationType = 1
)

type IntegrationEventType int64

const (
	IntegrationEventTypeAgentOnline  IntegrationEventType = 0
	IntegrationEventTypeAgentOffline IntegrationEventType = 1
	IntegrationEventTypePlayerJoined IntegrationEventType = 2
	IntegrationEventTypePlayerLeft   IntegrationEventType = 3
)

type AccountIntegrations struct {
	ID         primitive.ObjectID        `json:"_id" bson:"_id"`
	Type       IntegrationType           `json:"type" bson:"type"`
	Url        string                    `json:"url" bson:"url"`
	EventTypes []IntegrationEventType    `json:"eventTypes" bson:"eventTypes"`
	Events     []AccountIntegrationEvent `json:"events" bson:"events"`
	CreatedAt  time.Time                 `json:"createdAt" bson:"createdAt"`
	UpdatedAt  time.Time                 `json:"updatedAt" bson:"updatedAt"`
}

type AccountIntegrationEvent struct {
	ID           primitive.ObjectID   `json:"_id" bson:"_id"`
	Type         IntegrationEventType `json:"type" bson:"type"`
	Retries      int                  `json:"retries" bson:"retries"`
	Status       string               `json:"status" bson:"status"`
	Data         interface{}          `json:"data" bson:"data"`
	ResponseData interface{}          `json:"responseData" bson:"responseData"`
	Completed    bool                 `json:"completed" bson:"completed"`
	Failed       bool                 `json:"failed" bson:"failed"`
	CreatedAt    time.Time            `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time            `json:"updatedAt" bson:"updatedAt"`
}

func (obj *Accounts) AtomicDelete() error {
	if err := obj.PopulateUsers(); err != nil {
		return err
	}

	if err := obj.PopulateSessions(); err != nil {
		return err
	}

	if err := obj.PopulateAgents(); err != nil {
		return err
	}

	if err := obj.PopulateAudit(); err != nil {
		return err
	}

	fmt.Printf("* account contains: users: %d, sessions: %d, audit: %d, agents: %d\n", len(obj.UserObjects), len(obj.SessionObjects), len(obj.AuditObjects), len(obj.AgentObjects))

	for i := range obj.UserObjects {
		user := &obj.UserObjects[i]
		fmt.Printf("* deleting user: %s\n", user.Email)
		if err := user.AtomicDelete(); err != nil {
			return err
		}
	}

	for i := range obj.SessionObjects {
		session := &obj.SessionObjects[i]
		fmt.Printf("* deleting session: %s\n", session.ID.Hex())
		if err := session.AtomicDelete(); err != nil {
			return err
		}
	}

	for i := range obj.AuditObjects {
		audit := &obj.AuditObjects[i]
		fmt.Printf("* deleting audit: %s\n", audit.ID.Hex())
		if err := audit.AtomicDelete(); err != nil {
			return err
		}
	}
	for i := range obj.AgentObjects {
		agent := &obj.AgentObjects[i]
		fmt.Printf("* deleting agent: %s\n", agent.AgentName)
		if err := agent.AtomicDelete(); err != nil {
			return err
		}
	}
	if _, err := mongoose.DeleteOne(bson.M{"_id": obj.ID}, Accounts{}); err != nil {
		return err
	}

	fmt.Printf("deleted account: %s\n", obj.AccountName)

	return nil
}

func (obj *AccountSessions) AtomicDelete() error {

	if _, err := mongoose.DeleteOne(bson.M{"_id": obj.ID}, AccountSessions{}); err != nil {
		return err
	}

	return nil
}

func (obj *AccountAudit) AtomicDelete() error {

	if _, err := mongoose.DeleteOne(bson.M{"_id": obj.ID}, AccountAudit{}); err != nil {
		return err
	}

	return nil
}

func (obj *Accounts) PopulateFromURLQuery(populateStrings []string) error {
	for _, popStr := range populateStrings {
		if popStr == "integrations" {
			if err := obj.PopulateIntegrations(); err != nil {
				return err
			}
		}
	}

	return nil
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

func (obj *Accounts) PopulateIntegrations() error {

	err := mongoose.PopulateObjectArray(obj, "Integrations", &obj.IntegrationObjects)

	if err != nil {
		return err
	}

	if obj.IntegrationObjects == nil {
		obj.IntegrationObjects = make([]AccountIntegrations, 0)
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

func (obj *Accounts) CreateIntegrationEvent(eventType IntegrationEventType, data interface{}) error {
	if err := obj.PopulateIntegrations(); err != nil {
		return err
	}

	for idx := range obj.IntegrationObjects {
		integration := &obj.IntegrationObjects[idx]

		containsEventType := false
		for _, testEventType := range integration.EventTypes {
			if testEventType == eventType {
				containsEventType = true
				break
			}
		}

		if containsEventType {

			switch v := data.(type) {
			case EventDataAgentOnline:
				v.EventData.IntegrationId = integration.ID
				data = v
			case EventDataAgentOffline:
				v.EventData.IntegrationId = integration.ID
				data = v
			}

			if err := integration.AddEvent(eventType, data); err != nil {
				return err
			}
		}
	}

	return nil
}

func (obj *Accounts) ProcessIntegrationEvents() error {
	if err := obj.PopulateIntegrations(); err != nil {
		return err
	}

	for idx := range obj.IntegrationObjects {
		integration := &obj.IntegrationObjects[idx]
		if err := integration.ProcessEvents(); err != nil {
			return err
		}
	}

	return nil
}

func (obj *Accounts) AddIntegration(newIntegration AccountIntegrations) error {
	if newIntegration.ID.IsZero() {
		newIntegration.ID = primitive.NewObjectID()
		newIntegration.CreatedAt = time.Now()
		newIntegration.UpdatedAt = time.Now()
		newIntegration.Events = make([]AccountIntegrationEvent, 0)
	}

	if _, err := mongoose.InsertOne(&newIntegration); err != nil {
		return err
	}

	obj.Integrations = append(obj.Integrations, newIntegration.ID)

	dbUpdate := bson.D{{"$set", bson.D{
		{"integrations", obj.Integrations},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		return err
	}

	return nil
}

func (obj *Accounts) UpdateIntegration(updatedIntegration AccountIntegrations) error {
	if err := obj.PopulateIntegrations(); err != nil {
		return err
	}

	exists := false

	for _, integration := range obj.IntegrationObjects {
		if integration.ID.Hex() == updatedIntegration.ID.Hex() {
			exists = true
			break
		}
	}

	if !exists {
		return fmt.Errorf("error integration doesn't exist on account")
	}

	dbIntegration := AccountIntegrations{}
	if err := mongoose.FindOne(bson.M{"_id": updatedIntegration.ID}, &dbIntegration); err != nil {
		return err
	}

	dbIntegration.Type = updatedIntegration.Type
	dbIntegration.EventTypes = updatedIntegration.EventTypes
	dbIntegration.Url = updatedIntegration.Url

	dbUpdate := bson.D{{"$set", bson.D{
		{"type", dbIntegration.Type},
		{"eventTypes", dbIntegration.EventTypes},
		{"url", dbIntegration.Url},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(dbIntegration, dbUpdate); err != nil {
		return err
	}

	return nil
}

func (obj *Accounts) DeleteIntegration(integrationId primitive.ObjectID) error {

	if err := obj.PopulateIntegrations(); err != nil {
		return err
	}

	newArray := make(primitive.A, 0)

	for _, integration := range obj.IntegrationObjects {
		if integration.ID.Hex() != integrationId.Hex() {
			newArray = append(newArray, integration.ID)
		}
	}

	obj.Integrations = newArray

	dbUpdate := bson.D{{"$set", bson.D{
		{"integrations", obj.Integrations},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		return err
	}

	if _, err := mongoose.DeleteOne(bson.M{"_id": integrationId}, AccountIntegrations{}); err != nil {
		return err
	}

	return nil
}

func (obj *AccountIntegrations) AddEvent(eventType IntegrationEventType, data interface{}) error {

	newEvent := AccountIntegrationEvent{
		ID:        primitive.NewObjectID(),
		Type:      eventType,
		UpdatedAt: time.Now(),
		CreatedAt: time.Now(),
	}

	switch v := data.(type) {
	case EventDataAgentOnline:
		v.EventData.EventId = newEvent.ID
		data = v
	case EventDataAgentOffline:
		v.EventData.EventId = newEvent.ID
		data = v
	}

	newEvent.Data = data

	obj.Events = append(obj.Events, newEvent)

	dbUpdate := bson.D{{"$set", bson.D{
		{"events", obj.Events},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		return err
	}

	return nil
}

func (obj *AccountIntegrations) ProcessEvents() error {

	for idx := range obj.Events {
		event := &obj.Events[idx]

		if event.Completed {
			continue
		}

		if event.Failed {
			continue
		}

		if obj.Type == IntegrationWebhook {
			resp, err := ProcessWebhookEvent(obj.Url, event)
			if err != nil {
				event.Status = "failed"
				event.Retries += 1
				event.ResponseData = resp

				if event.Retries >= 10 {
					event.Failed = true
					event.Status = "failed - max retries"
				}
			} else {
				event.Completed = true
				event.Status = "delivered"
				event.ResponseData = resp
			}
		} else if obj.Type == IntegrationDiscord {
			if err := ProcessDiscordEvent(obj.Url, event); err != nil {
				event.Status = "failed"

				event.Retries += 1
				event.ResponseData = fmt.Sprintf(`{"success":false,"error":%s`, err.Error())

				if event.Retries >= 10 {
					event.Failed = true
					event.Status = "failed - max retries"
				}
			} else {
				event.Completed = true
				event.Status = "delivered"
				event.ResponseData = `{"success":true}`
			}
		}
	}

	dbUpdate := bson.D{{"$set", bson.D{
		{"events", obj.Events},
		{"updatedAt", time.Now()},
	}}}

	if err := mongoose.UpdateDataByID(*obj, dbUpdate); err != nil {
		return err
	}

	return nil
}

func ProcessWebhookEvent(url string, event *AccountIntegrationEvent) (string, error) {

	// Marshal the data into JSON
	jsonBytes, err := json.Marshal(event.Data)
	if err != nil {
		return "", err
	}

	// Prepare the webhook request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the webhook request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyString := string(bodyBytes)

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			logger.GetErrorLogger().Printf("error closing response body: %s", err)
		}
	}(resp.Body)

	// Determine the status based on the response code
	status := "failed"
	if resp.StatusCode == http.StatusOK {
		status = "delivered"
	}

	if status == "failed" {
		return bodyString, errors.New(status)
	}

	return bodyString, nil
}

func ProcessDiscordEvent(url string, event *AccountIntegrationEvent) error {

	EventNameStr := "SSM Event"
	fields := make([]discordwebhook.Field, 0)
	imageUrl := "https://ssmcloud.hostxtra.co.uk/public/images/ssm_logo256.png"

	eventTimeStr := ""

	switch event.Type {
	case IntegrationEventTypeAgentOnline:
		EventNameStr = "Agent Online"
		data := EventDataAgentOnline{}
		MarshalToEventData(event.Data, &data)

		fieldName := "AgentName"
		fieldValue := data.AgentName
		inline := true

		fields = append(fields, discordwebhook.Field{Name: &fieldName, Value: &fieldValue, Inline: &inline})
		eventTimeStr = data.EventData.EventTime.Format("2006-01-02 15:04:05")
	case IntegrationEventTypeAgentOffline:
		EventNameStr = "Agent Offline"
		data := EventDataAgentOffline{}
		MarshalToEventData(event.Data, &data)

		fieldName := "AgentName"
		fieldValue := data.AgentName
		inline := true

		fields = append(fields, discordwebhook.Field{Name: &fieldName, Value: &fieldValue, Inline: &inline})
		eventTimeStr = data.EventData.EventTime.Format("2006-01-02 15:04:05")
	}

	footer := discordwebhook.Footer{
		Text: &eventTimeStr,
	}

	embed := discordwebhook.Embed{
		Title:  &EventNameStr,
		Fields: &fields,
		Footer: &footer,
	}

	username := "SSM Cloud"
	message := discordwebhook.Message{
		Username:  &username,
		Embeds:    &[]discordwebhook.Embed{embed},
		AvatarUrl: &imageUrl,
	}

	if err := discordwebhook.SendMessage(url, message); err != nil {
		return err
	}

	return nil
}

func MarshalToEventData(data interface{}, output interface{}) {
	bodyBytes, _ := json.Marshal(data)
	json.Unmarshal(bodyBytes, output)
}
