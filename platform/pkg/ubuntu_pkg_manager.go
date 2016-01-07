package pkg

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type ubuntuPkgManager struct {
	cmdRunner boshsys.CmdRunner
}

func NewUbuntuPkgManager(cmdRunner boshsys.CmdRunner) Manager {
	return &ubuntuPkgManager{cmdRunner: cmdRunner}
}

func (u *ubuntuPkgManager) RemovePackage(packageName string) error {
	_, _, _, err := u.cmdRunner.RunCommand("apt-get", "-y", "remove", packageName)
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to apt-get remove")
	}

	_, _, _, err = u.cmdRunner.RunCommand("apt-get", "-y", "autoremove")
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to apt-get autoremove")
	}

	return nil
}
