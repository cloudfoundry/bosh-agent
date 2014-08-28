package disk

type PartitionType string

const (
	PartitionTypeSwap  PartitionType = "swap"
	PartitionTypeLinux PartitionType = "linux"
	PartitionTypeEmpty PartitionType = "empty"
)

type Partition struct {
	SizeInMb uint64
	Type     PartitionType
}

type Partitioner interface {
	Partition(devicePath string, partitions []Partition) (err error)
	GetDeviceSizeInMb(devicePath string) (size uint64, err error)
}

type RootDevicePartition struct {
	SizeInBytes uint64
}

type RootDevicePartitioner interface {
	PartitionAfterFirstPartition(devicePath string, partitions []RootDevicePartition) error
	GetRemainingSizeInBytes(devicePath string) (size uint64, err error)
}
