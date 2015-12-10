package net

import (
	"fmt"
	"strings"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type WindowsNetManager struct {
	psRunner boshsys.PSRunner

	logTag string
	logger boshlog.Logger
}

func NewWindowsNetManager(psRunner boshsys.PSRunner, logger boshlog.Logger) Manager {
	return WindowsNetManager{
		psRunner: psRunner,

		logTag: "WindowsNetManager",
		logger: logger,
	}
}

func (net WindowsNetManager) SetupNetworking(networks boshsettings.Networks, errCh chan error) error {
	const setDNSTemplate = `
[array]$interfaces = Get-DNSClientServerAddress
$dns = @("%s")
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ServerAddresses ($dns -join ",")
}
`

	const resetDNSTemplate = `
[array]$interfaces = Get-DNSClientServerAddress
foreach($interface in $interfaces) {
	Set-DnsClientServerAddress -InterfaceAlias $interface.InterfaceAlias -ResetServerAddresses
}
`

	nonVipNetworks := boshsettings.Networks{}

	for networkName, networkSettings := range networks {
		if networkSettings.IsVIP() {
			continue
		}
		nonVipNetworks[networkName] = networkSettings
	}

	dnsNetwork, _ := nonVipNetworks.DefaultNetworkFor("dns")

	if len(dnsNetwork.DNS) > 0 {
		_, _, err := net.psRunner.RunCommand(boshsys.PSCommand{
			Script: fmt.Sprintf(setDNSTemplate, strings.Join(dnsNetwork.DNS, `","`)),
		})
		if err != nil {
			return bosherr.WrapError(err, "Configuring DNS servers")
		}
	} else {
		_, _, err := net.psRunner.RunCommand(boshsys.PSCommand{Script: resetDNSTemplate})
		if err != nil {
			return bosherr.WrapError(err, "Resetting DNS servers")
		}
	}

	return nil
}

func (net WindowsNetManager) GetConfiguredNetworkInterfaces() ([]string, error) {
	panic("Not implemented")
}
