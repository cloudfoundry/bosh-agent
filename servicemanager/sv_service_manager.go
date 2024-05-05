package servicemanager

import (
	"path"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type svServiceManager struct {
	runner boshsys.CmdRunner
	fs     boshsys.FileSystem
}

func NewSvServiceManager(fs boshsys.FileSystem, runner boshsys.CmdRunner) ServiceManager {
	return &svServiceManager{
		fs:     fs,
		runner: runner,
	}
}

func (serviceManager svServiceManager) Kill(serviceName string) error {
	_, _, _, err := serviceManager.runner.RunCommand("sv", "kill", serviceName)
	return err
}

func (serviceManager svServiceManager) Setup(serviceName string) error {
	return serviceManager.fs.Symlink(path.Join("/etc", "sv", "monit"), path.Join("/etc", "service", "monit"))
}

func (serviceManager svServiceManager) Start(serviceName string) error {
	_, _, _, err := serviceManager.runner.RunCommand("sv", "start", serviceName)
	return err
}

func (serviceManager svServiceManager) Stop(serviceName string) error {
	_, _, _, err := serviceManager.runner.RunCommand("sv", "stop", serviceName)
	return err
}
