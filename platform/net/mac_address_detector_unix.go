// +build !windows

package net

import (
	"path"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const (
	ifaliasPrefix = "bosh-interface"
)

type linuxMacAddressDetector struct {
	fs boshsys.FileSystem
}

func NewMacAddressDetector(fs boshsys.FileSystem) MACAddressDetector {
	return linuxMacAddressDetector{
		fs: fs,
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
