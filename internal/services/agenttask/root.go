package agenttask

import (
	"context"
	"time"

	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/repositories"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/utils/logger"
	"github.com/mrhid6/go-mongoose-lock/joblock"
)

var reaperJob *joblock.JobLockTask

// InitAgentTaskService creates the indexes before anything can dispatch. If the
// indexes are missing, the queue's invariants are not enforced, so a failure here
// must stop the process rather than degrade silently.
func InitAgentTaskService() error {
	if err := EnsureIndexes(); err != nil {
		return err
	}

	var err error
	reaperJob, err = joblock.NewJobLockTask(
		repositories.GetMongoClient(),
		"reapAgentTaskLeasesJob",
		func() {
			if err := ReapExpiredLeases(); err != nil {
				logger.GetErrorLogger().Printf("error reaping expired task leases: %s", err.Error())
			}
		},
		10*time.Second,
		30*time.Second,
		false,
	)
	if err != nil {
		return err
	}

	if err := reaperJob.Run(context.Background()); err != nil {
		return err
	}

	StartDispatcher()

	logger.GetDebugLogger().Println("Initalized Agent Task Service")
	return nil
}

func ShutdownAgentTaskService() error {
	StopDispatcher()

	if reaperJob != nil {
		if err := reaperJob.UnLock(context.TODO()); err != nil {
			return err
		}
	}

	logger.GetDebugLogger().Println("Shutdown Agent Task Service")
	return nil
}
