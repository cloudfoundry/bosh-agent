package net

import (
	gonet "net"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type windowsMacAddressDetector struct{}

func NewMacAddressDetector(_ interface{}) MACAddressDetector {
	return windowsMacAddressDetector{}
}

func (d windowsMacAddressDetector) DetectMacAddresses() (map[string]string, error) {
	ifs, err := gonet.Interfaces()
	if err != nil {
		return nil, bosherr.WrapError(err, "Detecting Mac Addresses")
	}
	macs := make(map[string]string, len(ifs))
	for _, f := range ifs {
		macs[f.HardwareAddr.String()] = f.Name
	}
	return macs, nil
}
