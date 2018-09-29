package disk

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/clock"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type EphemeralDevicePartitioner struct {
	partedPartitioner Partitioner
	deviceUtil        Util
	logger            boshlog.Logger

	logTag      string
	cmdRunner   boshsys.CmdRunner
	fs          boshsys.FileSystem
	timeService clock.Clock
}

type Settings struct {
	AgentID string `json:"agent_id"`
}

func NewEphemeralDevicePartitioner(
	partedPartitioner Partitioner,
	deviceUtil Util,
	logger boshlog.Logger,
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	timeService clock.Clock,
) *EphemeralDevicePartitioner {
	return &EphemeralDevicePartitioner{
		partedPartitioner: partedPartitioner,
		deviceUtil:        deviceUtil,
		logger:            logger,
		logTag:            "EphemeralDevicePartitioner",
		cmdRunner:         cmdRunner,
		fs:                fs,
		timeService:       timeService,
	}
}

func (p *EphemeralDevicePartitioner) Partition(devicePath string, partitions []Partition) error {
	existingPartitions, deviceFullSizeInBytes, err := p.partedPartitioner.GetPartitions(devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting existing partitions of `%s'", devicePath)
	}

	if p.partitionsMatch(existingPartitions, partitions, deviceFullSizeInBytes) {
		p.logger.Info(p.logTag, "%s already partitioned as expected, skipping", devicePath)
		return nil
	}

	err = p.removePartitions(existingPartitions, devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Removing existing partitions of `%s'", devicePath)
	}

	err = p.ensureGPTPartition(devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Ensuring GPT table of `%s'", devicePath)
	}

	return p.partedPartitioner.Partition(devicePath, partitions)
}

func (p *EphemeralDevicePartitioner) GetDeviceSizeInBytes(devicePath string) (uint64, error) {
	return p.partedPartitioner.GetDeviceSizeInBytes(devicePath)
}

func (p *EphemeralDevicePartitioner) GetPartitions(devicePath string) (partitions []ExistingPartition, deviceFullSizeInBytes uint64, err error) {
	return p.partedPartitioner.GetPartitions(devicePath)
}

func (p *EphemeralDevicePartitioner) partitionsMatch(existingPartitions []ExistingPartition, desiredPartitions []Partition, deviceSizeInBytes uint64) bool {
	if len(existingPartitions) < len(desiredPartitions) {
		return false
	}

	remainingDiskSpace := deviceSizeInBytes

	for index, partition := range desiredPartitions {
		if index == len(desiredPartitions)-1 && partition.SizeInBytes == 0 {
			partition.SizeInBytes = remainingDiskSpace
		}

		existingPartition := existingPartitions[index]
		if existingPartition.Type != partition.Type {
			return false
		} else if !withinDelta(partition.SizeInBytes, existingPartition.SizeInBytes, p.convertFromMbToBytes(deltaSize)) {
			return false
		}

		if !strings.HasPrefix(existingPartition.Name, partition.NamePrefix) {
			return false
		}

		remainingDiskSpace = remainingDiskSpace - partition.SizeInBytes
	}

	return true
}

func (p EphemeralDevicePartitioner) removePartitions(partitions []ExistingPartition, devicePath string) error {
	partitionPaths, err := p.getPartitionPaths(devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting partition paths of disk `%s'", devicePath)
	}

	p.logger.Debug(p.logTag, "Erasing old partition paths")
	for _, partitionPath := range partitionPaths {
		partitionRetryable := boshretry.NewRetryable(func() (bool, error) {
			_, _, _, err := p.cmdRunner.RunCommand(
				"wipefs",
				"-a",
				partitionPath,
			)
			if err != nil {
				return true, bosherr.WrapError(err, fmt.Sprintf("Erasing partition path `%s' ", partitionPath))
			}

			p.logger.Info(p.logTag, "Successfully erased partition path `%s'", partitionPath)
			return false, nil
		})

		partitionRetryStrategy := NewPartitionStrategy(partitionRetryable, p.timeService, p.logger)
		err := partitionRetryStrategy.Try()

		if err != nil {
			return bosherr.WrapErrorf(err, "Erasing partition `%s' paths", devicePath)
		}
	}

	p.logger.Debug(p.logTag, "Removing old partitions")
	for _, partition := range partitions {
		partitionRetryable := boshretry.NewRetryable(func() (bool, error) {
			_, _, _, err := p.cmdRunner.RunCommand(
				"parted",
				devicePath,
				"rm",
				strconv.Itoa(partition.Index),
			)
			if err != nil {
				return true, bosherr.WrapError(err, "Removing partition using parted")
			}

			p.logger.Info(p.logTag, "Successfully removed partition %s from %s", partition.Name, devicePath)
			return false, nil
		})

		partitionRetryStrategy := NewPartitionStrategy(partitionRetryable, p.timeService, p.logger)
		err := partitionRetryStrategy.Try()

		if err != nil {
			return bosherr.WrapErrorf(err, "Removing partitions of disk `%s'", devicePath)
		}
	}
	return nil
}

func (p EphemeralDevicePartitioner) getPartitionPaths(devicePath string) ([]string, error) {
	stdout, _, _, err := p.cmdRunner.RunCommand("blkid")
	if err != nil {
		return []string{}, err
	}

	pathRegExp := devicePath + "."
	re := regexp.MustCompile(pathRegExp)
	match := re.FindAllString(stdout, -1)

	if nil == match {
		return []string{}, nil
	}

	return match, nil
}

func (p EphemeralDevicePartitioner) convertFromMbToBytes(sizeInMb uint64) uint64 {
	return sizeInMb * 1024 * 1024
}

func (p EphemeralDevicePartitioner) ensureGPTPartition(devicePath string) (err error) {
	stdout, _, _, err := p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")

	if !strings.Contains(stdout, "gpt") {
		p.logger.Debug(p.logTag, "Creating gpt table")
		stdout, _, _, err = p.cmdRunner.RunCommand(
			"parted",
			"-s",
			devicePath,
			"mklabel",
			"gpt",
		)

		if err != nil {
			return bosherr.WrapErrorf(err, "Parted making label")
		}
	}

	return nil
}
