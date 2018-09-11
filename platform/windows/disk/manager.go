package disk

import (
	"github.com/cloudfoundry/bosh-utils/system"
)

//go:generate counterfeiter -o fakes/fake_windows_disk_formatter.go . WindowsDiskFormatter

type WindowsDiskFormatter interface {
	Format(diskNumber, partitionNumber string) error
}

//go:generate counterfeiter -o fakes/fake_windows_disk_linker.go . WindowsDiskLinker

type WindowsDiskLinker interface {
	LinkTarget(location string) (target string, err error)
	Link(location, target string) error
}

//go:generate counterfeiter -o fakes/fake_windows_disk_partitioner.go . WindowsDiskPartitioner

type WindowsDiskPartitioner interface {
	GetCountOnDisk(diskNumber string) (string, error)
	GetFreeSpaceOnDisk(diskNumber string) (int, error)
}

type Manager struct {
	formatter   WindowsDiskFormatter
	linker      WindowsDiskLinker
	partitioner WindowsDiskPartitioner
}

func NewWindowsDiskManager(cmdRunner system.CmdRunner) *Manager {
	formatter := &Formatter{
		Runner: cmdRunner,
	}

	linker := &Linker{
		Runner: cmdRunner,
	}

	partitioner := &Partitioner{
		Runner: cmdRunner,
	}

	return &Manager{
		formatter:   formatter,
		linker:      linker,
		partitioner: partitioner,
	}
}

func (m *Manager) GetFormatter() WindowsDiskFormatter {
	return m.formatter
}

func (m *Manager) GetLinker() WindowsDiskLinker {
	return m.linker
}

func (m *Manager) GetPartitioner() WindowsDiskPartitioner {
	return m.partitioner
}
