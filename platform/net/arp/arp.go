package arp

import (
	"fmt"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const arpLogTag = "arp"

type Arp struct {
	cmdRunner boshsys.CmdRunner
	logger    boshlog.Logger
}

func NewArp(
	cmdRunner boshsys.CmdRunner,
	logger boshlog.Logger,
) Arp {
	return Arp{
		cmdRunner: cmdRunner,
		logger:    logger,
	}
}

func (a Arp) Delete(address string) {
	a.logger.Debug(arpLogTag, fmt.Sprintf("Deleting %s from ARP cache", address))

	_, _, _, err := a.cmdRunner.RunCommand("arp", "-d", address)
	if err != nil {
		a.logger.Info(arpLogTag, "Ignoring arp failure deleting %s from cache: %s", address, err.Error())
	}
}
