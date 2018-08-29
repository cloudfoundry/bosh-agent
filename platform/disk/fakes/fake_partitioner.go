package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
)

type FakePartitioner struct {
	PartitionCalled     bool
	PartitionDevicePath string
	PartitionPartitions []boshdisk.Partition
	PartitionErr        error

	GetDeviceSizeInBytesDevicePath string
	GetDeviceSizeInBytesSizes      map[string]uint64
	GetDeviceSizeInBytesErr        error

	GetPartitionsPartitions []boshdisk.ExistingPartition
	GetPartitionsSizes      map[string]uint64
	GetPartitionsErr        error
}

func NewFakePartitioner() *FakePartitioner {
	return &FakePartitioner{
		GetDeviceSizeInBytesSizes: make(map[string]uint64),
	}
}

func (p *FakePartitioner) Partition(devicePath string, partitions []boshdisk.Partition) error {
	p.PartitionCalled = true
	p.PartitionDevicePath = devicePath
	p.PartitionPartitions = partitions
	return p.PartitionErr
}

func (p *FakePartitioner) GetDeviceSizeInBytes(devicePath string) (uint64, error) {
	p.GetDeviceSizeInBytesDevicePath = devicePath
	return p.GetDeviceSizeInBytesSizes[devicePath], p.GetDeviceSizeInBytesErr
}

func (p *FakePartitioner) GetPartitions(devicePath string) (partitions []boshdisk.ExistingPartition, deviceFullSizeInBytes uint64, err error) {
	return p.GetPartitionsPartitions, p.GetPartitionsSizes[devicePath], p.GetPartitionsErr
}
