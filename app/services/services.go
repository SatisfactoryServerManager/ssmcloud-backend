package services

var (
	_quit = make(chan int)
)

func InitAllServices() {
	InitStorageService()
	InitModService()
	InitAgentService()
	InitAccountService();
}

func ShutdownAllServices() error {

	_quit <- 0

	if err := ShutdownModService(); err != nil {
		return err
	}

	if err := ShutdownAgentService(); err != nil {
		return err
	}

	return nil
}
