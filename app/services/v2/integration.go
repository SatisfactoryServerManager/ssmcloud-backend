package v2

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/app/utils/logger"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
	"github.com/gtuk/discordwebhook"
	"github.com/mrhid6/go-mongoose-lock/joblock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	processIntegrationsJob *joblock.JobLockTask
	leaseTime              = 30 * time.Second
	maxAttempts            = 5
	podName, _             = os.Hostname()
)

func InitIntegrationService() error {

	processIntegrationsJob, _ = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"processIntegrationsJob", func() {
			if err := processAllIntegrationEvents(); err != nil {
				fmt.Println(err)
			}
		},
		1*time.Minute,
		10*time.Second,
		false,
	)

	ctx := context.Background()

	if err := processIntegrationsJob.Run(ctx); err != nil {
		logger.GetErrorLogger().Printf("%v\n", err.Error())
	}

	return nil
}

func ShutdownIntegrationService() error {
	processIntegrationsJob.UnLock(context.TODO())

	logger.GetDebugLogger().Println("Shutdown Integration Service")
	return nil
}

func GetMyAccountIntegrationsEvents(integrationId primitive.ObjectID) ([]v2.IntegrationEventSchema, error) {

	IntegrationEventModel, err := repositories.GetMongoClient().GetModel("IntegrationEvent")
	if err != nil {
		return nil, err
	}

	events := make([]v2.IntegrationEventSchema, 0)

	findOptions := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	if err := IntegrationEventModel.FindAllWithOptions(&events, bson.M{"integrationId": integrationId}, findOptions); err != nil {
		return nil, err
	}

	return events, nil
}

func AddIntegrationEvent(theAccount *v2.AccountSchema, eventType v2.IntegrationEventType, payload interface{}) error {
	integrations, err := GetAccountIntegrationsWithEventType(theAccount, eventType)

	if err != nil {
		return err
	}

	for _, integration := range integrations {
		if err := createIntegrationEvent(integration, eventType, payload); err != nil {
			return err
		}
	}

	return nil
}

func GetAccountIntegrationsWithEventType(theAccount *v2.AccountSchema, eventType v2.IntegrationEventType) ([]*v2.AccountIntegrationSchema, error) {

	AccountModel, err := repositories.GetMongoClient().GetModel("Account")
	if err != nil {
		return nil, err
	}

	if err := AccountModel.PopulateField(theAccount, "Integrations"); err != nil {
		return nil, fmt.Errorf("error populating account integrations with error: %s", err.Error())
	}

	contains := func(list []v2.IntegrationEventType, target v2.IntegrationEventType) bool {
		for _, v := range list {
			if v == target {
				return true
			}
		}
		return false
	}

	res := make([]*v2.AccountIntegrationSchema, 0)
	for idx := range theAccount.Integrations {
		integration := &theAccount.Integrations[idx]

		if contains(integration.EventTypes, eventType) {
			res = append(res, integration)
		}
	}

	return res, nil
}

func createIntegrationEvent(accountIntegration *v2.AccountIntegrationSchema, eventType v2.IntegrationEventType, payload interface{}) error {
	IntegrationEventModel, err := repositories.GetMongoClient().GetModel("IntegrationEvent")
	if err != nil {
		return err
	}

	if accountIntegration.Type == v2.IntegrationDiscord {
		data, err := BuildDiscordEventPayload(eventType, payload)
		if err != nil {
			return err
		}
		payload = data
	}

	data, err := structToStringMap(payload)
	if err != nil {
		return fmt.Errorf("error converting payload to string map with error: %s", err.Error())
	}

	newIntegrationEvent := v2.NewIntegrationEvent(accountIntegration, eventType, data)

	if err := IntegrationEventModel.Create(newIntegrationEvent); err != nil {
		return fmt.Errorf("error creating integration event with error: %s", err.Error())
	}
	return nil
}

func BuildDiscordEventPayload(eventType v2.IntegrationEventType, payload interface{}) (*discordwebhook.Message, error) {

	imageUrl := "https://ssmcloud.hostxtra.co.uk/public/images/ssm_logo256.png"
	username := "SSM Cloud"
	EventNameStr := "SSM Event"
	fields := make([]discordwebhook.Field, 0)

	data, err := structToStringMap(payload)
	if err != nil {
		return nil, fmt.Errorf("error converting payload to string map with error: %s", err.Error())
	}

	for key, value := range data {
		inline := true
		val := value.(string)
		fields = append(fields, discordwebhook.Field{Name: &key, Value: &val, Inline: &inline})
	}

	switch eventType {
	case v2.IntegrationEventTypeAgentCreated:
		EventNameStr = "Server Created"
	case v2.IntegrationEventTypeAgentRemoved:
		EventNameStr = "Server Removed"
	case v2.IntegrationEventTypeAgentOnline:
		EventNameStr = "Server Online"
	case v2.IntegrationEventTypeAgentOffline:
		EventNameStr = "Server Offline"
	case v2.IntegrationEventTypeUserAdded:
		EventNameStr = "User Added To Account"
	case v2.IntegrationEventTypeUserRemoved:
		EventNameStr = "User Removed From Account"
	case v2.IntegrationEventTypePlayerJoined:
		EventNameStr = "Player Joined The Server"
	case v2.IntegrationEventTypePlayerLeft:
		EventNameStr = "Player Left The Server"
	default:
		EventNameStr = "Unknown SSM Event"
	}

	embed := discordwebhook.Embed{
		Title:  &EventNameStr,
		Fields: &fields,
	}

	message := &discordwebhook.Message{
		Username:  &username,
		Embeds:    &[]discordwebhook.Embed{embed},
		AvatarUrl: &imageUrl,
	}
	return message, nil
}

func structToStringMap(input interface{}) (map[string]interface{}, error) {
	// Marshal struct to JSON
	bytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON into a generic map
	var result map[string]interface{}
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func processAllIntegrationEvents() error {

	ev, err := claimEvent()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := processEvent(ev); err != nil {
		markFailed(ev, err)
	} else {
		markSent(ev)
	}
	return nil
}

func claimEvent() (*v2.IntegrationEventSchema, error) {

	IntegrationEventModel, err := repositories.GetMongoClient().GetModel("IntegrationEvent")
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	leaseUntil := now.Add(leaseTime)
	filter := bson.M{
		"status":          "pending",
		"next_attempt_at": bson.M{"$lte": now},
		"$or": []bson.M{
			{"processing_until": bson.M{"$exists": false}},
			{"processing_until": bson.M{"$lte": now}},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"status":           "processing",
			"processing_by":    podName,
			"processing_until": leaseUntil,
		},
		"$inc": bson.M{"attempts": 1},
	}

	ev := &v2.IntegrationEventSchema{}

	if err := IntegrationEventModel.FindOneAndUpdate(ev, filter, update); err != nil {
		return nil, err
	}
	return ev, nil
}

func processEvent(ev *v2.IntegrationEventSchema) error {
	payload, _ := json.Marshal(ev.Payload)
	req, err := http.NewRequestWithContext(context.Background(), "POST", ev.URL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ev.Response = string(bodyBytes)
	ev.ResponseCode = resp.StatusCode

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, ev.URL)
	}
	return nil
}

func markSent(ev *v2.IntegrationEventSchema) {

	IntegrationEventModel, err := repositories.GetMongoClient().GetModel("IntegrationEvent")
	if err != nil {
		log.Printf("markSent error: %v", err)
		return
	}

	now := time.Now().UTC()

	update := bson.M{
		"$set": bson.M{
			"status":       "sent",
			"response":     ev.Response,
			"responseCode": ev.ResponseCode,
			"sent_at":      now,
			"last_error":   "",
		},
		"$unset": bson.M{"processing_by": "", "processing_until": ""},
	}

	if err := IntegrationEventModel.RawUpdateData(ev, update); err != nil {
		log.Printf("markSent error: %v", err)
	}
}

func markFailed(ev *v2.IntegrationEventSchema, err error) {

	IntegrationEventModel, merr := repositories.GetMongoClient().GetModel("IntegrationEvent")
	if merr != nil {
		log.Printf("markFailed error: %v", merr)
		return
	}

	nextDelay := time.Duration(ev.Attempts*ev.Attempts) * time.Second // exponential backoff
	if nextDelay > 2*time.Minute {
		nextDelay = 2 * time.Minute
	}
	nextTime := time.Now().Add(nextDelay)

	status := "pending"
	if ev.Attempts >= maxAttempts {
		status = "failed"
	}

	update := bson.M{
		"$set": bson.M{
			"status":          status,
			"last_error":      err.Error(),
			"response":        ev.Response,
			"responseCode":    ev.ResponseCode,
			"next_attempt_at": nextTime,
		},
		"$unset": bson.M{"processing_by": "", "processing_until": ""},
	}

	if uerr := IntegrationEventModel.RawUpdateData(ev, update); uerr != nil {
		log.Printf("markFailed update error: %v", uerr)
	}
}
