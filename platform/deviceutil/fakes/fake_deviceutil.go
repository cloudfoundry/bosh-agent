package fakes

type FakeDeviceUtil struct {
	GetFileContentsFilename string
	GetFileContentsError    error
	GetFileContentsContents []byte
}

func NewFakeDeviceUtil() (util *FakeDeviceUtil) {
	util = &FakeDeviceUtil{}
	return
}

func (util *FakeDeviceUtil) GetFileContents(fileName string) ([]byte, error) {
	util.GetFileContentsFilename = fileName
	return util.GetFileContentsContents, util.GetFileContentsError
}
