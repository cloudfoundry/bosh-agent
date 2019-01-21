package disk

import (
	"errors"

	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const MaxFdiskPartitionSize = uint64(2 * 1024 * 1024 * 1024 * 1024)

var ErrGPTPartitionEncountered = errors.New("sfdisk detected a GPT partition")

type PersistentDevicePartitioner struct {
	sfDiskPartitioner Partitioner
	partedPartitioner Partitioner
	deviceUtil        Util
	logger            logger.Logger
	runner            boshsys.CmdRunner
}

func NewPersistentDevicePartitioner(
	sfDiskPartitioner Partitioner,
	partedPartitioner Partitioner,
	deviceUtil Util,
	logger logger.Logger,
	runner boshsys.CmdRunner,
) *PersistentDevicePartitioner {
	return &PersistentDevicePartitioner{
		sfDiskPartitioner: sfDiskPartitioner,
		partedPartitioner: partedPartitioner,
		deviceUtil:        deviceUtil,
		logger:            logger,
		runner:            runner,
	}
}

func (p *PersistentDevicePartitioner) Partition(devicePath string, partitions []Partition) error {
	// Reread partitions of the new block device (ioctl BLKRRPART)
	// See https://github.com/cloudfoundry/bosh-agent/issues/198
	_, _, _, err := p.runner.RunCommand("blockdev", "--rereadpt", devicePath)

	if err != nil {
		p.logger.Debug("persistent-disk-partitioner", "Reread partitions of '%s'", devicePath)
		return err
	}

	size, err := p.deviceUtil.GetBlockDeviceSize(devicePath)
	if err != nil {
		p.logger.Debug("persistent-disk-partitioner", "Attempting to get block device size")
		return err
	}

	if size > MaxFdiskPartitionSize {
		p.logger.Debug("persistent-disk-partitioner", "Using parted partitioner because disk size is too large: %d", size)
		return p.partedPartitioner.Partition(devicePath, partitions)
	}

	p.logger.Debug("persistent-disk-partitioner", "Attempting to partition with sfdisk partitioner")
	err = p.sfDiskPartitioner.Partition(devicePath, partitions)
	if IsGPTError(err) {
		p.logger.Debug("persistent-disk-partitioner", "GPT partition detected, falling back to parted")
		return p.partedPartitioner.Partition(devicePath, partitions)
	}

	return err
}

func (p *PersistentDevicePartitioner) GetDeviceSizeInBytes(devicePath string) (uint64, error) {
	return p.sfDiskPartitioner.GetDeviceSizeInBytes(devicePath)
}

func IsGPTError(err error) bool {
	return err == ErrGPTPartitionEncountered
}
