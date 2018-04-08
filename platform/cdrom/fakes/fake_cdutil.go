package fakes

type FakeCDUtil struct {
	GetFilesContentsFileNames []string
	GetFilesContentsError     error
	GetFilesContentsContents  [][]byte

	GetBlockDeviceSizeError error
	GetBlockDeviceSizeSize  uint64
}

func NewFakeCDUtil() (util *FakeCDUtil) {
	util = &FakeCDUtil{}
	return
}

func (util *FakeCDUtil) GetFilesContents(fileNames []string) ([][]byte, error) {
	util.GetFilesContentsFileNames = fileNames
	return util.GetFilesContentsContents, util.GetFilesContentsError
}

func (util *FakeCDUtil) GetBlockDeviceSize() (size uint64, err error) {
	return util.GetBlockDeviceSizeSize, util.GetBlockDeviceSizeError
}
