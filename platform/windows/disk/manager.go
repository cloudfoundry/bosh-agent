package disk

import (
	"github.com/cloudfoundry/bosh-utils/system"

	"github.com/cloudfoundry/bosh-agent/v2/platform/windows/powershell"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o fakes/fake_windows_disk_formatter.go . WindowsDiskFormatter

type WindowsDiskFormatter interface {
	Format(diskNumber, partitionNumber string) error
}

//counterfeiter:generate -o fakes/fake_windows_disk_linker.go . WindowsDiskLinker

type WindowsDiskLinker interface {
	LinkTarget(location string) (target string, err error)
	Link(location, target string) error
}

//counterfeiter:generate -o fakes/fake_windows_disk_partitioner.go . WindowsDiskPartitioner

type WindowsDiskPartitioner interface {
	GetCountOnDisk(diskNumber string) (string, error)
	GetFreeSpaceOnDisk(diskNumber string) (int, error)
	InitializeDisk(diskNumber string) error
	PartitionDisk(diskNumber string) (string, error)
	AssignDriveLetter(diskNumber, partitionNumber string) (string, error)
}

//counterfeiter:generate -o fakes/fake_windows_disk_protector.go . WindowsDiskProtector

type WindowsDiskProtector interface {
	CommandExists() bool
	ProtectPath(path string) error
}

type Manager struct {
	formatter   WindowsDiskFormatter
	linker      WindowsDiskLinker
	partitioner WindowsDiskPartitioner
	protector   WindowsDiskProtector
}

func NewWindowsDiskManager(cmdRunner system.CmdRunner) *Manager {
	var runner system.CmdRunner

	switch cmdRunner.(type) {
	case *powershell.Runner:
		runner = cmdRunner
	default:
		runner = &powershell.Runner{
			BaseCmdRunner: cmdRunner,
		}
	}

	formatter := &Formatter{
		Runner: runner,
	}

	linker := &Linker{
		Runner: runner,
	}

	partitioner := &Partitioner{
		Runner: runner,
	}

	protector := &Protector{
		Runner: runner,
	}

	return &Manager{
		formatter:   formatter,
		linker:      linker,
		partitioner: partitioner,
		protector:   protector,
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

func (m *Manager) GetProtector() WindowsDiskProtector {
	return m.protector
}
