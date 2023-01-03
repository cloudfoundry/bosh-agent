package disk

type FileSystemType string

const (
	FileSystemSwap    FileSystemType = "swap"
	FileSystemBTRFS   FileSystemType = "btrfs"
	FileSystemExt4    FileSystemType = "ext4"
	FileSystemXFS     FileSystemType = "xfs"
	FileSystemDefault FileSystemType = ""

	FileSystemBTRFSResizeUtility = "btrfs"
	FileSystemExtResizeUtility   = "resize2fs"
	FileSystemXFSResizeUtility   = "xfs_growfs"
)

type Formatter interface {
	Format(partitionPath string, fsType FileSystemType) (err error)
	GetPartitionFormatType(string) (FileSystemType, error)
	GrowFilesystem(partitionPath string) error
}
