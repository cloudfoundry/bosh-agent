package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type FakeDevicePathResolver struct {
	GetRealDevicePathDiskSettings []boshsettings.DiskSettings
	RealDevicePath                string
	GetRealDevicePathStub         func(boshsettings.DiskSettings) (string, bool, error)
	GetRealDevicePathTimedOut     bool
	GetRealDevicePathErr          error
}

func NewFakeDevicePathResolver() *FakeDevicePathResolver {
	return &FakeDevicePathResolver{}
}

func (r *FakeDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	r.GetRealDevicePathDiskSettings = append(r.GetRealDevicePathDiskSettings, diskSettings)

	if r.GetRealDevicePathStub != nil {
		return r.GetRealDevicePathStub(diskSettings)
	}

	if r.GetRealDevicePathErr != nil {
		return "", r.GetRealDevicePathTimedOut, r.GetRealDevicePathErr
	}

	return r.RealDevicePath, false, nil
}
