package disk

type Manager interface {
	GetEphemeralDevicePartitioner() Partitioner
	GetFormatter() Formatter
	GetMounter() Mounter
	GetMountsSearcher() MountsSearcher
	GetPersistentDevicePartitioner() Partitioner
	GetRootDevicePartitioner() Partitioner
	GetUtil() Util
}
