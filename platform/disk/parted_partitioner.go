package disk

import (
	"fmt"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type partedPartitioner struct {
	logger    boshlog.Logger
	cmdRunner boshsys.CmdRunner
	logTag    string
}

func NewPartedPartitioner(logger boshlog.Logger, cmdRunner boshsys.CmdRunner) partedPartitioner {
	return partedPartitioner{
		logger:    logger,
		cmdRunner: cmdRunner,
		logTag:    "PartedPartitioner",
	}
}

type existingPartition struct {
	Index        int
	SizeInBytes  uint64
	StartInBytes uint64
	EndInBytes   uint64
}

func (p partedPartitioner) PartitionAfterFirstPartition(devicePath string, partitions []RootDevicePartition) error {
	existingPartitions, err := p.getPartitions(devicePath)
	if err != nil {
		return bosherr.WrapError(err, "Partitioning disk `%s'", devicePath)
	}

	if len(existingPartitions) == 0 {
		return bosherr.New("Missing first partition on `%s'", devicePath)
	}

	partitionStart := existingPartitions[0].EndInBytes

	for index, partition := range partitions {
		partitionEnd := partitionStart + partition.SizeInBytes

		if len(existingPartitions) > index+1 {
			existingPartition := existingPartitions[index+1]

			if partition.SizeInBytes == existingPartition.SizeInBytes {
				partitionStart = partitionEnd
				p.logger.Info(p.logTag, "Skipping partition %d because it already exists", index)
				continue
			} else {
				err = p.removePartitions(devicePath, existingPartitions[index+1:])
				if err != nil {
					return bosherr.WrapError(err, "Partitioning disk `%s'", devicePath)
				}
				existingPartitions = existingPartitions[:index+1]
			}
		}

		p.logger.Info(p.logTag, "Creating partition %d with start %d and end %d", index, partitionStart, partitionEnd)

		_, _, _, err := p.cmdRunner.RunCommand(
			"parted",
			"-s",
			devicePath,
			"unit",
			"B",
			"mkpart",
			"primary",
			fmt.Sprintf("%d", partitionStart),
			fmt.Sprintf("%d", partitionEnd),
		)

		if err != nil {
			return bosherr.WrapError(err, "Partitioning disk `%s'", devicePath)
		}

		partitionStart = partitionEnd
	}
	return nil
}

func (p partedPartitioner) GetRemainingSizeInBytes(devicePath string) (uint64, error) {
	p.logger.Debug(p.logTag, "Getting size of disk remaining after first partition")

	stdout, _, _, err := p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting remaining size of `%s'", devicePath)
	}

	allLines := strings.Split(stdout, "\n")
	if len(allLines) < 3 {
		return 0, bosherr.New("Getting remaining size of `%s'", devicePath)
	}

	partitionInfoLines := allLines[1:3]
	deviceInfo := strings.Split(partitionInfoLines[0], ":")
	deviceFullSizeInBytes, err := strconv.ParseUint(strings.TrimRight(deviceInfo[1], "B"), 10, 64)
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting remaining size of `%s'", devicePath)
	}

	firstPartitionInfo := strings.Split(partitionInfoLines[1], ":")
	firstPartitionEndInBytes, err := strconv.ParseUint(strings.TrimRight(firstPartitionInfo[2], "B"), 10, 64)
	if err != nil {
		return 0, bosherr.WrapError(err, "Getting remaining size of `%s'", devicePath)
	}

	remainingSizeInBytes := deviceFullSizeInBytes - firstPartitionEndInBytes

	return remainingSizeInBytes, nil
}

func (p partedPartitioner) getPartitions(devicePath string) ([]existingPartition, error) {
	partitions := []existingPartition{}

	stdout, _, _, err := p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")
	if err != nil {
		return partitions, bosherr.WrapError(err, "Getting existing partitions of `%s'", devicePath)
	}

	p.logger.Debug(p.logTag, "Found partitions %s", stdout)

	allLines := strings.Split(stdout, "\n")
	if len(allLines) < 3 {
		return partitions, bosherr.New("Parsing existing partitions of `%s'", devicePath)
	}

	partitionLines := allLines[2 : len(allLines)-1]

	for _, partitionLine := range partitionLines {
		partitionInfo := strings.Split(partitionLine, ":")
		partitionIndex, err := strconv.Atoi(partitionInfo[0])
		if err != nil {
			return partitions, bosherr.WrapError(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitionStartInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[1], "B"))
		if err != nil {
			return partitions, bosherr.WrapError(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitionEndInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[2], "B"))
		if err != nil {
			return partitions, bosherr.WrapError(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitionSizeInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[3], "B"))
		if err != nil {
			return partitions, bosherr.WrapError(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitions = append(
			partitions,
			existingPartition{
				Index:        partitionIndex,
				SizeInBytes:  uint64(partitionSizeInBytes),
				StartInBytes: uint64(partitionStartInBytes),
				EndInBytes:   uint64(partitionEndInBytes),
			},
		)
	}

	return partitions, nil
}

func (p partedPartitioner) removePartitions(devicePath string, partitions []existingPartition) error {
	for _, partition := range partitions {
		p.logger.Info(p.logTag, "Removing partition %d", partition.Index)

		_, _, _, err := p.cmdRunner.RunCommand("parted", "-s", devicePath, "rm", fmt.Sprintf("%d", partition.Index))

		if err != nil {
			return bosherr.WrapError(err, "Removing partition from `%s'", devicePath)
		}
	}

	return nil
}
