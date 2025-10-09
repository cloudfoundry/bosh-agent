package net

import (
	"encoding/json"
	gonet "net"
	"path"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . MACAddressDetector

type MACAddressDetector interface {
	DetectMacAddresses() (map[string]string, error)
}

const (
	ifaliasPrefix            = "bosh-interface"
	sriovVFIfalias           = "sriov-vf"
	macAddressDetectorLogTag = "MacAddressDetector"
)

type linuxMacAddressDetector struct {
	fs     boshsys.FileSystem
	logger boshlog.Logger
}

type windowsMacAddressDetector struct {
	interfacesFunction func() ([]gonet.Interface, error)
	runner             boshsys.CmdRunner
}

type netAdapter struct {
	Name       string
	MacAddress string
}

func NewLinuxMacAddressDetector(fs boshsys.FileSystem, logger boshlog.Logger) MACAddressDetector {
	return linuxMacAddressDetector{
		fs:     fs,
		logger: logger,
	}
}

func NewWindowsMacAddressDetector(runner boshsys.CmdRunner, interfacesFunction func() ([]gonet.Interface, error)) MACAddressDetector {
	return windowsMacAddressDetector{
		interfacesFunction: interfacesFunction,
		runner:             runner,
	}
}

func (d linuxMacAddressDetector) DetectMacAddresses() (map[string]string, error) {
	addresses := map[string]string{}

	filePaths, err := d.fs.Glob("/sys/class/net/*")
	if err != nil {
		return addresses, bosherr.WrapError(err, "Getting file list from /sys/class/net")
	}

	var macAddress string
	var ifalias string
	for _, filePath := range filePaths {
		isPhysicalDevice := d.fs.FileExists(path.Join(filePath, "device"))

		// For third-party networking plugin case that the physical interface is used as bridge
		// interface and a virtual interface is created to replace it, the virtual interface needs
		// to be included in the detected result.
		// The virtual interface has an ifalias that has the prefix "bosh-interface"
		hasBoshPrefix := false
		ifalias, err = d.fs.ReadFileString(path.Join(filePath, "ifalias"))
		if err == nil {
			hasBoshPrefix = strings.HasPrefix(ifalias, ifaliasPrefix)
		}

		// SR-IOV VF interfaces share the same MAC address as synthetic interfaces and should be ignored
		// They have an ifalias with the content "sriov-vf"
		isSRIOVVF := false
		if err == nil {
			isSRIOVVF = strings.Contains(ifalias, sriovVFIfalias)
			if isSRIOVVF {
				interfaceName := path.Base(filePath)
				d.logger.Debug(macAddressDetectorLogTag, "Ignoring SR-IOV VF interface: %s (ifalias: %s)", interfaceName, strings.TrimSpace(ifalias))
				continue
			}
		}

		if isPhysicalDevice || hasBoshPrefix {
			macAddress, err = d.fs.ReadFileString(path.Join(filePath, "address"))
			if err != nil {
				return addresses, bosherr.WrapError(err, "Reading mac address from file")
			}

			macAddress = strings.Trim(macAddress, "\n")

			interfaceName := path.Base(filePath)
			addresses[macAddress] = interfaceName
		}
	}

	return addresses, nil
}

func (d windowsMacAddressDetector) DetectMacAddresses() (map[string]string, error) {
	ifs, err := d.interfacesFunction()
	if err != nil {
		return nil, bosherr.WrapError(err, "Detecting Mac Addresses")
	}
	macs := make(map[string]string, len(ifs))

	var netAdapters []netAdapter
	stdout, stderr, _, err := d.runner.RunCommand("powershell", "-Command", "Get-NetAdapter | Select MacAddress,Name | ConvertTo-Json")
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Getting visible adapters: %s", stderr)
	}

	err = json.Unmarshal([]byte(stdout), &netAdapters)
	if err != nil {
		var singularNetAdapter netAdapter
		err = json.Unmarshal([]byte(stdout), &singularNetAdapter)
		if err != nil {
			return nil, bosherr.WrapError(err, "Parsing Get-NetAdapter output")
		}
		netAdapters = append(netAdapters, singularNetAdapter)
	}

	for _, f := range ifs {
		if adapterVisible(netAdapters, f.HardwareAddr.String(), f.Name) {
			macs[f.HardwareAddr.String()] = f.Name
		}
	}
	return macs, nil
}

func adapterVisible(netAdapters []netAdapter, macAddress string, adapterName string) bool {
	for _, adapter := range netAdapters {
		adapterMac, _ := gonet.ParseMAC(adapter.MacAddress) //nolint:errcheck
		if adapter.Name == adapterName && adapterMac.String() == macAddress {
			return true
		}
	}
	return false
}
