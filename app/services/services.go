package services

import v2 "github.com/SatisfactoryServerManager/ssmcloud-backend/app/services/v2"

func InitAllServices() {
	InitStorageService()
	InitModService()
	InitAgentService()
	InitAccountService()
	v2.InitIntegrationService()
}

func ShutdownAllServices() error {

	if err := ShutdownModService(); err != nil {
		return err
	}

	if err := ShutdownAgentService(); err != nil {
		return err
	}

	if err := ShutdownAccountService(); err != nil {
		return err
	}

	if err := v2.ShutdownIntegrationService(); err != nil {
		return err
	}

	return nil
}
