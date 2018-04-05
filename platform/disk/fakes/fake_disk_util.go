package fakes

type FakeDiskUtil struct {
	GetFilesContentsFileNames []string
	GetFilesContentsError     error
	GetFilesContentsContents  [][]byte
	GetFilesContentsDiskPath  string

	GetBlockDeviceSizeDiskPath string
	GetBlockDeviceSizeError    error
	GetBlockDeviceSizeSize     uint64
}

func NewFakeDiskUtil() (util *FakeDiskUtil) {
	util = &FakeDiskUtil{}
	return
}

func (util *FakeDiskUtil) GetFilesContents(diskPath string, fileNames []string) ([][]byte, error) {
	util.GetFilesContentsDiskPath = diskPath
	util.GetFilesContentsFileNames = fileNames
	return util.GetFilesContentsContents, util.GetFilesContentsError
}

func (util *FakeDiskUtil) GetBlockDeviceSize(diskPath string) (size uint64, err error) {
	util.GetBlockDeviceSizeDiskPath = diskPath
	return util.GetBlockDeviceSizeSize, util.GetBlockDeviceSizeError
}
