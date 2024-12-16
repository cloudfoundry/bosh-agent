package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
)

type FakePartitioner struct {
	SinglePartitionNeedsResizeCalled                bool
	SinglePartitionNeedsResizeDevicePath            string
	SinglePartitionNeedsResizeExpectedPartitionType boshdisk.PartitionType
	SinglePartitionNeedsResizeReturns               struct {
		NeedResize bool
		Err        error
	}

	ResizeSinglePartitionCalled     bool
	ResizeSinglePartitionDevicePath string
	ResizeSinglePartitionErr        error

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

func (p *FakePartitioner) SinglePartitionNeedsResize(devicePath string, expectedPartitionType boshdisk.PartitionType) (needsResize bool, err error) {
	p.SinglePartitionNeedsResizeCalled = true
	p.SinglePartitionNeedsResizeDevicePath = devicePath
	p.SinglePartitionNeedsResizeExpectedPartitionType = expectedPartitionType
	return p.SinglePartitionNeedsResizeReturns.NeedResize, p.SinglePartitionNeedsResizeReturns.Err
}

func (p *FakePartitioner) ResizeSinglePartition(devicePath string) (err error) {
	p.ResizeSinglePartitionCalled = true
	p.ResizeSinglePartitionDevicePath = devicePath
	return p.ResizeSinglePartitionErr
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
