package deviceutil

type DeviceUtil interface {
	GetFileContents(fileName string) (contents []byte, err error)
}
