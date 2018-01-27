package platform

import boshsys "github.com/cloudfoundry/bosh-utils/system"

var _ boshsys.FileSystem = &DummyFs{}

func DummyWrapFs(fs boshsys.FileSystem) boshsys.FileSystem {
	return &DummyFs{
		FileSystem: fs,
	}
}

type DummyFs struct {
	boshsys.FileSystem
}

func (d *DummyFs) Chown(path string, username string) error {
	return nil
}
