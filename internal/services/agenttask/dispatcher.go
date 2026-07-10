package agenttask

import (
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const dispatchTick = 500 * time.Millisecond

var (
	wake     = make(chan bson.ObjectID, 64)
	dispDone = make(chan struct{})
)

// notifyEnqueued wakes the dispatcher immediately when the target agent's stream
// lives on this replica. Otherwise the owning replica's tick picks the task up
// within dispatchTick. The fast path is an optimization; the tick is what makes
// it correct.
func notifyEnqueued(agentID bson.ObjectID) {
	if !registry.Has(agentID) {
		return
	}
	select {
	case wake <- agentID:
	default: // wake channel full; the tick will catch it
	}
}

func StartDispatcher() {
	go func() {
		ticker := time.NewTicker(dispatchTick)
		defer ticker.Stop()

		for {
			select {
			case agentID := <-wake:
				dispatchFor(agentID)
			case <-ticker.C:
				for _, agentID := range registry.ConnectedAgents() {
					dispatchFor(agentID)
				}
			case <-dispDone:
				return
			}
		}
	}()

	logger.GetDebugLogger().Println("Started agent task dispatcher")
}

func StopDispatcher() {
	close(dispDone)
	logger.GetDebugLogger().Println("Stopped agent task dispatcher")
}

// dispatchFor claims at most one task for the agent and pushes it. Claim returns
// (nil, nil) when the agent is busy or nothing is due, so this is safe to call
// as often as we like.
func dispatchFor(agentID bson.ObjectID) {
	task, err := Claim(agentID)
	if err != nil {
		logger.GetErrorLogger().Printf("error claiming task for agent %s: %s", agentID.Hex(), err.Error())
		return
	}
	if task == nil {
		return
	}

	a := Assignment{
		TaskID:       task.ID.Hex(),
		Action:       task.Action,
		Data:         task.Data,
		LeaseToken:   task.LeaseToken,
		Attempt:      int32(task.Attempts),
		MaxAttempts:  int32(task.MaxAttempts),
		LeaseSeconds: int32(LeaseDuration.Seconds()),
	}

	if registry.send(agentID, a) {
		logger.GetDebugLogger().Printf("dispatched task %s (%s) to agent %s", a.TaskID, a.Action, agentID.Hex())
		return
	}

	// The stream vanished between Claim and send. Put the task back immediately
	// rather than waiting out the lease.
	if err := Release(a.TaskID, a.LeaseToken); err != nil {
		logger.GetErrorLogger().Printf("error releasing undeliverable task %s: %s", a.TaskID, err.Error())
	}
}
