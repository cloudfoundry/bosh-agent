package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
)

type FakeFormatter struct {
	GetFileSystemType    map[string]boshdisk.FileSystemType
	FormatCalled         bool
	FormatPartitionPaths []string
	FormatFsTypes        []boshdisk.FileSystemType
	FormatError          error
}

func NewFakeFormatter() *FakeFormatter {
	return &FakeFormatter{
		GetFileSystemType: make(map[string]boshdisk.FileSystemType),
	}
}

func (p *FakeFormatter) Format(partitionPath string, fsType boshdisk.FileSystemType) (err error) {
	if p.FormatError != nil {
		return p.FormatError
	}
	p.FormatCalled = true
	p.FormatPartitionPaths = append(p.FormatPartitionPaths, partitionPath)
	p.FormatFsTypes = append(p.FormatFsTypes, fsType)
	return
}

func (p *FakeFormatter) GetPartitionFormatType(partitionPath string) (boshdisk.FileSystemType, error) {
	if p.FormatError != nil {
		return "", p.FormatError
	}
	return p.GetFileSystemType[partitionPath], nil
}
