package fakes

type FakeBundleInstallCallBack func()

type FakeBundle struct {
	ActionsCalled []string

	InstallSourcePath   string
	InstallPathInBundle string
	InstallCallBack     FakeBundleInstallCallBack
	InstallPath         string
	InstallError        error
	Installed           bool

	IsInstalledErr error

	GetDirPath  string
	GetDirError error

	EnablePath  string
	EnableError error
	Enabled     bool

	DisableErr error

	UninstallErr error
}

func NewFakeBundle() (bundle *FakeBundle) {
	bundle = &FakeBundle{
		ActionsCalled: []string{},
	}
	return
}

func (s *FakeBundle) Install(sourcePath, pathInBundle string) (string, error) {
	s.InstallSourcePath = sourcePath
	s.InstallPathInBundle = pathInBundle
	s.Installed = true
	s.ActionsCalled = append(s.ActionsCalled, "Install")
	if s.InstallCallBack != nil {
		s.InstallCallBack()
	}
	return s.InstallPath, s.InstallError
}

func (s *FakeBundle) InstallWithoutContents() (string, error) {
	s.Installed = true
	s.ActionsCalled = append(s.ActionsCalled, "InstallWithoutContents")
	return s.InstallPath, s.InstallError
}

func (s *FakeBundle) GetInstallPath() (string, error) {
	return s.GetDirPath, s.GetDirError
}

func (s *FakeBundle) IsInstalled() (bool, error) {
	return s.Installed, s.IsInstalledErr
}

func (s *FakeBundle) Enable() (string, error) {
	s.Enabled = true
	s.ActionsCalled = append(s.ActionsCalled, "Enable")
	return s.EnablePath, s.EnableError
}

func (s *FakeBundle) Disable() error {
	s.ActionsCalled = append(s.ActionsCalled, "Disable")
	return s.DisableErr
}

func (s *FakeBundle) Uninstall() error {
	s.ActionsCalled = append(s.ActionsCalled, "Uninstall")
	return s.UninstallErr
}
