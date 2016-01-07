package fakes

type FakeManager struct {
	RemovedPackages []string
}

func NewFakePackageManager() *FakeManager {
	return &FakeManager{
		RemovedPackages: []string{},
	}
}

func (f *FakeManager) RemovePackage(packageName string) error {
	f.RemovedPackages = append(f.RemovedPackages, packageName)
	return nil
}
