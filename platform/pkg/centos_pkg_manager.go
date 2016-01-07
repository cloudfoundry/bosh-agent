package pkg

import (
	"fmt"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type centosPkgManager struct {
	cmdRunner boshsys.CmdRunner
}

func NewCentosPkgManager(cmdRunner boshsys.CmdRunner) Manager {
	return &centosPkgManager{cmdRunner: cmdRunner}
}

func (u *centosPkgManager) RemovePackage(packageName string) error {
	_, _, _, err := u.cmdRunner.RunCommand("sh", "-c", fmt.Sprintf("yum -y remove --disablerepo='*' %s", packageName))
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to yum remove")
	}

	return nil
}
