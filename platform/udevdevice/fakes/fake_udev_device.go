package fakes

type FakeUdevDevice struct {
	KickDeviceFile string

	Settled   bool
	SettleErr error

	Triggered  bool
	TriggerErr error

	EnsureDeviceReadableFile  string
	EnsureDeviceReadableError error
}

func NewFakeUdevDevice() *FakeUdevDevice {
	return &FakeUdevDevice{}
}

func (l *FakeUdevDevice) KickDevice(filePath string) {
	l.KickDeviceFile = filePath
}

func (l *FakeUdevDevice) Settle() (err error) {
	l.Settled = true
	return l.SettleErr
}

func (l *FakeUdevDevice) Trigger() (err error) {
	l.Triggered = true
	return l.TriggerErr
}

func (l *FakeUdevDevice) EnsureDeviceReadable(filePath string) (err error) {
	err = l.EnsureDeviceReadableError
	l.EnsureDeviceReadableFile = filePath
	return
}
