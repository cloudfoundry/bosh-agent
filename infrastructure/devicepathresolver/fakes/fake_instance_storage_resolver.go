package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type FakeInstanceStorageResolver struct {
	DiscoverInstanceStorageDevices   []boshsettings.DiskSettings
	DiscoverInstanceStoragePaths     []string
	DiscoverInstanceStorageErr       error
	DiscoverInstanceStorageCallCount int
	DiscoverInstanceStorageStub      func([]boshsettings.DiskSettings) ([]string, error)
}

func NewFakeInstanceStorageResolver() *FakeInstanceStorageResolver {
	return &FakeInstanceStorageResolver{}
}

func (r *FakeInstanceStorageResolver) DiscoverInstanceStorage(devices []boshsettings.DiskSettings) ([]string, error) {
	r.DiscoverInstanceStorageDevices = devices
	r.DiscoverInstanceStorageCallCount++

	if r.DiscoverInstanceStorageStub != nil {
		return r.DiscoverInstanceStorageStub(devices)
	}

	if r.DiscoverInstanceStorageErr != nil {
		return nil, r.DiscoverInstanceStorageErr
	}

	return r.DiscoverInstanceStoragePaths, nil
}
