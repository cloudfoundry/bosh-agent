package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
)

type FakeFormatter struct {
	FormatCalled         bool
	FormatPartitionPaths []string
	FormatFsTypes        []boshdisk.FileSystemType
	FormatError          error

	GrowFilesystemCalled        bool
	GrowFilesystemPartitionPath string
	GrowFilesystemError         error
}

func (p *FakeFormatter) Format(partitionPath string, fsType boshdisk.FileSystemType) (err error) {
	p.FormatCalled = true
	p.FormatPartitionPaths = append(p.FormatPartitionPaths, partitionPath)
	p.FormatFsTypes = append(p.FormatFsTypes, fsType)
	return p.FormatError
}

func (p *FakeFormatter) GrowFilesystem(partitionPath string) error {
	p.GrowFilesystemCalled = true
	p.GrowFilesystemPartitionPath = partitionPath
	return p.GrowFilesystemError
}
