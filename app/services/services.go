package services

func InitAllServices() {
	InitModService()
}

func ShutdownAllServices() error {

	if err := ShutdownModService(); err != nil {
		return err
	}

	return nil
}
