package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
)

type FakePartitioner struct {
	PartitionsNeedResizeReturns struct {
		NeedResize bool
		Err        error
	}

	ResizePartitionsCalled     bool
	ResizePartitionsDevicePath string
	ResizePartitionsPartitions []boshdisk.Partition
	ResizePartitionsErr        error

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

	RemovePartitionsCalled  bool
	RemoveExistingPartition []boshdisk.ExistingPartition
	RemovePartitionsErr     error
}

func NewFakePartitioner() *FakePartitioner {
	return &FakePartitioner{
		GetDeviceSizeInBytesSizes: make(map[string]uint64),
	}
}

func (p *FakePartitioner) PartitionsNeedResize(devicePath string, partitions []boshdisk.Partition) (needsResize bool, err error) {
	return p.PartitionsNeedResizeReturns.NeedResize, p.PartitionsNeedResizeReturns.Err
}

func (p *FakePartitioner) ResizePartitions(devicePath string, partitions []boshdisk.Partition) (err error) {
	p.ResizePartitionsCalled = true
	p.ResizePartitionsDevicePath = devicePath
	p.ResizePartitionsPartitions = partitions
	return p.ResizePartitionsErr
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

func (p *FakePartitioner) RemovePartitions(partitions []boshdisk.ExistingPartition, devicePath string) error {
	p.RemovePartitionsCalled = true
	p.RemoveExistingPartition = partitions
	p.PartitionDevicePath = devicePath
	return p.RemovePartitionsErr
}
