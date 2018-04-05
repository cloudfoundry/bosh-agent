package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
)

type FakeDiskManager struct {
	FakeEphemeralPartitioner  *FakePartitioner
	FakePersistentPartitioner *FakePartitioner
	FakeFormatter             *FakeFormatter
	FakeMounter               *FakeMounter
	FakeMountsSearcher        *FakeMountsSearcher
	FakeRootDevicePartitioner *FakePartitioner
	FakeDiskUtil              *FakeDiskUtil
}

func NewFakeDiskManager() *FakeDiskManager {
	return &FakeDiskManager{
		FakeEphemeralPartitioner:  NewFakePartitioner(),
		FakePersistentPartitioner: NewFakePartitioner(),
		FakeFormatter:             &FakeFormatter{},
		FakeMounter:               &FakeMounter{},
		FakeMountsSearcher:        &FakeMountsSearcher{},
		FakeRootDevicePartitioner: NewFakePartitioner(),
		FakeDiskUtil:              NewFakeDiskUtil(),
	}
}

func (m *FakeDiskManager) GetRootDevicePartitioner() boshdisk.Partitioner {
	return m.FakeRootDevicePartitioner
}

func (m *FakeDiskManager) GetEphemeralDevicePartitioner() boshdisk.Partitioner {
	return m.FakeEphemeralPartitioner
}

func (m *FakeDiskManager) GetPersistentDevicePartitioner() boshdisk.Partitioner {
	return m.FakePersistentPartitioner
}

func (m *FakeDiskManager) GetFormatter() boshdisk.Formatter {
	return m.FakeFormatter
}

func (m *FakeDiskManager) GetMounter() boshdisk.Mounter {
	return m.FakeMounter
}

func (m *FakeDiskManager) GetMountsSearcher() boshdisk.MountsSearcher {
	return m.FakeMountsSearcher
}

func (m *FakeDiskManager) GetUtil() boshdisk.Util {
	return m.FakeDiskUtil
}
