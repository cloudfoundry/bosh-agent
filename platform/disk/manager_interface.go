package disk

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Manager

type Manager interface {
	GetEphemeralDevicePartitioner() Partitioner
	GetFormatter() Formatter
	GetMounter() Mounter
	GetMountsSearcher() MountsSearcher
	GetPersistentDevicePartitioner(partitionerType string) (Partitioner, error)
	GetRootDevicePartitioner() Partitioner
	GetUtil() Util
}
