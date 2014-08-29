package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
)

type FakePartitioner struct {
	PartitionCalled     bool
	PartitionDevicePath string
	PartitionPartitions []boshdisk.Partition

	GetDeviceSizeInBytesSizes map[string]uint64
}

func (p *FakePartitioner) Partition(devicePath string, partitions []boshdisk.Partition) (err error) {
	p.PartitionCalled = true
	p.PartitionDevicePath = devicePath
	p.PartitionPartitions = partitions
	return
}

func (p *FakePartitioner) GetDeviceSizeInBytes(devicePath string) (size uint64, err error) {
	size = p.GetDeviceSizeInBytesSizes[devicePath]
	return
}

type FakeRootDevicePartitioner struct {
	DevicePathCalled                string
	PartitionsCalled                []boshdisk.RootDevicePartition
	PartitionAfterFirstPartitionErr error

	GetRemainingSizeInMbDevicePath string
	GetRemainingSizeInBytesSizes   map[string]uint64
	GetRemainingSizeInMbErr        error
}

func NewFakeRootDevicePartitioner() *FakeRootDevicePartitioner {
	return &FakeRootDevicePartitioner{
		GetRemainingSizeInBytesSizes: make(map[string]uint64),
	}
}

func (p *FakeRootDevicePartitioner) PartitionAfterFirstPartition(devicePath string, partitions []boshdisk.RootDevicePartition) error {
	p.DevicePathCalled = devicePath
	p.PartitionsCalled = partitions
	return p.PartitionAfterFirstPartitionErr
}

func (p *FakeRootDevicePartitioner) GetRemainingSizeInBytes(devicePath string) (uint64, error) {
	p.GetRemainingSizeInMbDevicePath = devicePath
	return p.GetRemainingSizeInBytesSizes[devicePath], p.GetRemainingSizeInMbErr
}
