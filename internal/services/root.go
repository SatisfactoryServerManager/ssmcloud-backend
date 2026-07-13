package services

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agentmod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agenttask"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/integration"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/mod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/storage"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/workflow"
)

func InitAllServices() {
	storage.InitStorageService()
	agent.InitAgentService()

	// agentmod.Init() ensures indexes and runs the modConfig backfill. It must
	// finish before agenttask.InitAgentTaskService() starts the dispatcher: a
	// dispatched syncmods task assumes agentmods already holds every agent's
	// selection, and the backfill is what puts it there.
	if err := agentmod.Init(); err != nil {
		panic(err)
	}

	if err := agenttask.InitAgentTaskService(); err != nil {
		panic(err)
	}
	account.InitAccountService()

	mod.InitModService()
	workflow.InitWorkflowService()
	integration.InitIntegrationService()
}

func ShutdownAllServices() error {

	if err := mod.ShutdownModService(); err != nil {
		return err
	}

	if err := agent.ShutdownAgentService(); err != nil {
		return err
	}

	if err := account.ShutdownAccountService(); err != nil {
		return err
	}

	if err := agenttask.ShutdownAgentTaskService(); err != nil {
		return err
	}

	if err := workflow.ShutdownWorkflowService(); err != nil {
		return err
	}

	if err := integration.ShutdownIntegrationService(); err != nil {
		return err
	}

	return nil
}
