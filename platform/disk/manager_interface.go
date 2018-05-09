package disk

//go:generate counterfeiter . Manager

type Manager interface {
	GetEphemeralDevicePartitioner() Partitioner
	GetFormatter() Formatter
	GetMounter() Mounter
	GetMountsSearcher() MountsSearcher
	GetPersistentDevicePartitioner(partitionerType string) (Partitioner, error)
	GetRootDevicePartitioner() Partitioner
	GetUtil() Util
}
