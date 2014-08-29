package disk

type PartitionType string

const (
	PartitionTypeSwap  PartitionType = "swap"
	PartitionTypeLinux PartitionType = "linux"
	PartitionTypeEmpty PartitionType = "empty"
)

type Partition struct {
	SizeInBytes uint64
	Type        PartitionType
}

type Partitioner interface {
	Partition(devicePath string, partitions []Partition) (err error)
	GetDeviceSizeInBytes(devicePath string) (size uint64, err error)
}
