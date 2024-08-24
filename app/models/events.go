package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventData struct {
	IntegrationId primitive.ObjectID `json:"integrationId"`
	EventId       primitive.ObjectID `json:"eventId"`
	EventType     string             `json:"eventType"`
	EventTime     time.Time          `json:"eventTime"`
}

type EventDataBasic struct{
    EventData `json:"eventData"`
}

type EventDataAgentOnline struct {
	EventData `json:"eventData"`
	AgentName string `json:"agentName"`
}

type EventDataAgentOffline struct {
	EventData `json:"eventData"`
	AgentName string `json:"agentName"`
}
