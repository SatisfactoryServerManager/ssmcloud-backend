package services

import v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"

func InitAllServices() {
	InitStorageService()
	v2.InitModService()
	InitAgentService()
	InitAccountService()
	v2.InitWorkflowService()
	v2.InitIntegrationService()
}

func ShutdownAllServices() error {

	if err := v2.ShutdownModService(); err != nil {
		return err
	}

	if err := ShutdownAgentService(); err != nil {
		return err
	}

	if err := ShutdownAccountService(); err != nil {
		return err
	}

	if err := v2.ShutdownWorkflowService(); err != nil {
		return err
	}

	if err := v2.ShutdownIntegrationService(); err != nil {
		return err
	}

	ShutdownGRPCServices()

	return nil
}
