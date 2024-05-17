package servicemanager

type dummyServiceManager struct {
}

func NewDummyServiceManager() ServiceManager {
	return &dummyServiceManager{}
}

func (serviceManager dummyServiceManager) Kill(serviceName string) error {
	return nil
}

func (serviceManager dummyServiceManager) Setup(serviceName string) error {
	return nil
}

func (serviceManager dummyServiceManager) Start(serviceName string) error {
	return nil
}

func (serviceManager dummyServiceManager) Stop(serviceName string) error {
	return nil
}
