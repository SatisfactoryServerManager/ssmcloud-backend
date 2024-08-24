package services

func InitAllServices() {
	InitStorageService()
	InitModService()
	InitAgentService()
	InitAccountService()
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

	return nil
}
