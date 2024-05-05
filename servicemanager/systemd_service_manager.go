package servicemanager

import boshsys "github.com/cloudfoundry/bosh-utils/system"

type systemdServiceManager struct {
	runner boshsys.CmdRunner
}

func NewSystemdServiceManager(runner boshsys.CmdRunner) ServiceManager {
	return &systemdServiceManager{
		runner: runner,
	}
}

func (serviceManager systemdServiceManager) Kill(serviceName string) error {
	_, _, _, err := serviceManager.runner.RunCommand("systemctl", "kill", serviceName)
	return err
}

func (serviceManager systemdServiceManager) Setup(serviceName string) error {
	return nil
}

func (serviceManager systemdServiceManager) Start(serviceName string) error {
	_, _, _, err := serviceManager.runner.RunCommand("systemctl", "start", serviceName)
	return err
}

func (serviceManager systemdServiceManager) Stop(serviceName string) error {
	_, _, _, err := serviceManager.runner.RunCommand("systemctl", "stop", serviceName)
	return err
}
