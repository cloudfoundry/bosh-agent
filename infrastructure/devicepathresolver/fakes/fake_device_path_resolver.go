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
	GetRealDevicePathCallCount_   int
	GetRealDevicePathReturnsPath  string
	GetRealDevicePathReturnsTO    bool
	GetRealDevicePathReturnsErr   error
}

func NewFakeDevicePathResolver() *FakeDevicePathResolver {
	return &FakeDevicePathResolver{}
}

func (r *FakeDevicePathResolver) GetRealDevicePath(diskSettings boshsettings.DiskSettings) (string, bool, error) {
	r.GetRealDevicePathDiskSettings = append(r.GetRealDevicePathDiskSettings, diskSettings)
	r.GetRealDevicePathCallCount_++

	if r.GetRealDevicePathStub != nil {
		return r.GetRealDevicePathStub(diskSettings)
	}

	if r.GetRealDevicePathReturnsErr != nil {
		return r.GetRealDevicePathReturnsPath, r.GetRealDevicePathReturnsTO, r.GetRealDevicePathReturnsErr
	}

	if r.GetRealDevicePathErr != nil {
		return "", r.GetRealDevicePathTimedOut, r.GetRealDevicePathErr
	}

	return r.RealDevicePath, false, nil
}

func (r *FakeDevicePathResolver) GetRealDevicePathCallCount() int {
	return r.GetRealDevicePathCallCount_
}

func (r *FakeDevicePathResolver) GetRealDevicePathReturns(path string, timedOut bool, err error) {
	r.GetRealDevicePathReturnsPath = path
	r.GetRealDevicePathReturnsTO = timedOut
	r.GetRealDevicePathReturnsErr = err
}
