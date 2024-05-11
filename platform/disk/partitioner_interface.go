package disk

import "fmt"

type PartitionType string

const (
	PartitionTypeSwap    PartitionType = "swap"
	PartitionTypeEFI     PartitionType = "efi"
	PartitionTypeLinux   PartitionType = "linux"
	PartitionTypeEmpty   PartitionType = "empty"
	PartitionTypeUnknown PartitionType = "unknown"
	PartitionTypeGPT     PartitionType = "gpt"
)

type Partition struct {
	NamePrefix  string
	SizeInBytes uint64
	Type        PartitionType
}

type Partitioner interface {
	Partition(devicePath string, partitions []Partition) (err error)
	GetDeviceSizeInBytes(devicePath string) (size uint64, err error)
	SinglePartitionNeedsResize(devicePath string, expectedPartitionType PartitionType) (needsResize bool, err error)
	ResizeSinglePartition(devicePath string) (err error)
	GetPartitions(devicePath string) (partitions []ExistingPartition, deviceFullSizeInBytes uint64, err error)
	RemovePartitions(partitions []ExistingPartition, devicePath string) error
}

func (p Partition) String() string {
	return fmt.Sprintf("[Type: %s, SizeInBytes: %d]", p.Type, p.SizeInBytes)
}
