package disk

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/clock"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type linuxDiskManager struct {
	ephemeralPartitioner  Partitioner
	partedPartitioner     Partitioner
	sfDiskPartitioner     Partitioner
	persistentPartitioner Partitioner
	rootDevicePartitioner Partitioner
	diskUtil              Util

	formatter Formatter

	mounter        Mounter
	mountsSearcher MountsSearcher

	fs     boshsys.FileSystem
	logger boshlog.Logger
	runner boshsys.CmdRunner
}

type LinuxDiskManagerOpts struct {
	BindMount       bool
	PartitionerType string
}

func NewLinuxDiskManager(
	logger boshlog.Logger,
	runner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	opts LinuxDiskManagerOpts,
) Manager {
	var mounter Mounter
	var mountsSearcher MountsSearcher

	// By default we want to use most reliable source of
	// mount information which is /proc/mounts
	mountsSearcher = NewProcMountsSearcher(fs)

	// Bind mounting in a container (warden) will not allow
	// reliably determine which device backs a mount point,
	// so we use less reliable source of mount information:
	// the mount command which returns information from /etc/mtab.
	if opts.BindMount {
		mountsSearcher = NewCmdMountsSearcher(runner)
	}

	mounter = NewLinuxMounter(runner, mountsSearcher, 1*time.Second)

	if opts.BindMount {
		mounter = NewLinuxBindMounter(mounter)
	}

	var ephemeralPartitioner, persistentPartitioner Partitioner

	diskUtil := NewUtil(runner, mounter, fs, logger)
	partedPartitioner := NewPartedPartitioner(logger, runner, clock.NewClock())
	sfDiskPartitioner := NewSfdiskPartitioner(logger, runner, clock.NewClock())

	switch opts.PartitionerType {
	case "parted":
		ephemeralPartitioner = NewEphemeralDevicePartitioner(partedPartitioner, logger, runner)
		persistentPartitioner = partedPartitioner
	case "sfdisk":
		ephemeralPartitioner = sfDiskPartitioner
		persistentPartitioner = sfDiskPartitioner
	case "":
		ephemeralPartitioner = NewEphemeralDevicePartitioner(partedPartitioner, logger, runner)
		persistentPartitioner = NewPersistentDevicePartitioner(sfDiskPartitioner, partedPartitioner, diskUtil, logger)
	default:
		panic(fmt.Sprintf("Unknown partitioner type '%s'", opts.PartitionerType))
	}

	return linuxDiskManager{
		ephemeralPartitioner:  ephemeralPartitioner,
		diskUtil:              diskUtil,
		formatter:             NewLinuxFormatter(runner, fs),
		fs:                    fs,
		logger:                logger,
		mounter:               mounter,
		mountsSearcher:        mountsSearcher,
		partedPartitioner:     partedPartitioner,
		sfDiskPartitioner:     sfDiskPartitioner,
		persistentPartitioner: persistentPartitioner,
		rootDevicePartitioner: NewRootDevicePartitioner(logger, runner, uint64(20*1024*1024)),
		runner:                runner,
	}
}

func (m linuxDiskManager) GetRootDevicePartitioner() Partitioner      { return m.rootDevicePartitioner }
func (m linuxDiskManager) GetEphemeralDevicePartitioner() Partitioner { return m.ephemeralPartitioner }

func (m linuxDiskManager) GetPersistentDevicePartitioner(partitionerType string) (Partitioner, error) {
	switch partitionerType {
	case "parted":
		return m.partedPartitioner, nil
	case "sfdisk":
		return m.sfDiskPartitioner, nil
	case "":
		return m.persistentPartitioner, nil
	default:
		return nil, fmt.Errorf("Unknown partitioner type '%s'", partitionerType) //nolint:staticcheck
	}
}

func (m linuxDiskManager) GetFormatter() Formatter           { return m.formatter }
func (m linuxDiskManager) GetMounter() Mounter               { return m.mounter }
func (m linuxDiskManager) GetMountsSearcher() MountsSearcher { return m.mountsSearcher }

func (m linuxDiskManager) GetUtil() Util { return m.diskUtil }
