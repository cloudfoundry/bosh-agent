package disk

type Manager interface {
	GetPartitioner() Partitioner
	GetRootDevicePartitioner() RootDevicePartitioner
	GetFormatter() Formatter
	GetMounter() Mounter
	GetMountsSearcher() MountsSearcher
}
