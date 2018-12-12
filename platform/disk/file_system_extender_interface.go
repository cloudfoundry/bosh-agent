package disk

type FileSystemExtender interface {
	Extend(partitionPath string, size uint64) (err error)
}
