package services

import (
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/account"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/agent"
	"github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/storage"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/internal/services/v2"
)

func InitAllServices() {
	storage.InitStorageService()
    agent.InitAgentService()
	account.InitAccountService()
    
	v2.InitModService()
	v2.InitWorkflowService()
	v2.InitIntegrationService()
}

func ShutdownAllServices() error {

	if err := v2.ShutdownModService(); err != nil {
		return err
	}

	if err := agent.ShutdownAgentService(); err != nil {
		return err
	}

	if err := account.ShutdownAccountService(); err != nil {
		return err
	}

	if err := v2.ShutdownWorkflowService(); err != nil {
		return err
	}

	if err := v2.ShutdownIntegrationService(); err != nil {
		return err
	}

	return nil
}
