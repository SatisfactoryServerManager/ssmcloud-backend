package services

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/integration"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/mod"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/storage"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/workflow"
)

func InitAllServices() {
	storage.InitStorageService()
	agent.InitAgentService()
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

	if err := workflow.ShutdownWorkflowService(); err != nil {
		return err
	}

	if err := integration.ShutdownIntegrationService(); err != nil {
		return err
	}

	return nil
}
