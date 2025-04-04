package disk

import (
	"fmt"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type rootDevicePartitioner struct {
	logger       boshlog.Logger
	cmdRunner    boshsys.CmdRunner
	deltaInBytes uint64
	logTag       string
}

func NewRootDevicePartitioner(logger boshlog.Logger, cmdRunner boshsys.CmdRunner, deltaInBytes uint64) Partitioner {
	return rootDevicePartitioner{
		logger:       logger,
		cmdRunner:    cmdRunner,
		deltaInBytes: deltaInBytes,
		logTag:       "RootDevicePartitioner",
	}
}

func (p rootDevicePartitioner) Partition(devicePath string, partitions []Partition) error {
	existingPartitions, deviceFullSizeInBytes, err := p.GetPartitions(devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting existing partitions of `%s'", devicePath)
	}
	p.logger.Debug(p.logTag, "Current partitions: %#v", existingPartitions)

	if len(existingPartitions) == 0 {
		return bosherr.Errorf("Missing first partition on `%s'", devicePath)
	}

	// If EFI based stemcells, there is an extra partition. Matching is accounting for it here.
	indexExistingStart := 1
	if existingPartitions[0].Type == PartitionTypeEFI {
		indexExistingStart = 2
	}

	if p.partitionsMatch(existingPartitions[indexExistingStart:], partitions) {
		p.logger.Info(p.logTag, "Partitions already match, skipping partitioning")
		return nil
	}

	// To support optimal reads on HDDs and optimal erasure on SSD: use 1MiB partition alignments.
	alignmentInBytes := uint64(1048576)

	partitionStart := p.roundUp(existingPartitions[len(existingPartitions)-1].EndInBytes+1, alignmentInBytes)

	for index, partition := range partitions {
		partitionEnd := partitionStart + partition.SizeInBytes - 1
		if partitionEnd >= deviceFullSizeInBytes {
			partitionEnd = deviceFullSizeInBytes - 1
			p.logger.Info(p.logTag, "Partition %d would be larger than remaining space. Reducing size to %dB", index, partitionEnd-partitionStart)
		}

		p.logger.Info(p.logTag, "Creating partition %d with start %dB and end %dB", index, partitionStart, partitionEnd)

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
			return bosherr.WrapErrorf(err, "Partitioning disk `%s'", devicePath)
		}

		partitionStart = p.roundUp(partitionEnd+1, alignmentInBytes)
	}
	return nil
}

func (p rootDevicePartitioner) GetDeviceSizeInBytes(devicePath string) (uint64, error) {
	p.logger.Debug(p.logTag, "Getting size of disk remaining after first partition")

	stdout, _, _, err := p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")
	if err != nil {
		return 0, bosherr.WrapErrorf(err, "Getting remaining size of `%s'", devicePath)
	}

	allLines := strings.Split(stdout, "\n")
	if len(allLines) < 3 {
		return 0, bosherr.Errorf("Getting remaining size of `%s'", devicePath)
	}

	partitionInfoLines := allLines[1 : len(allLines)-1]
	deviceInfo := strings.Split(partitionInfoLines[0], ":")
	deviceFullSizeInBytes, err := strconv.ParseUint(strings.TrimRight(deviceInfo[1], "B"), 10, 64)
	if err != nil {
		return 0, bosherr.WrapErrorf(err, "Getting remaining size of `%s'", devicePath)
	}

	firstPartitionInfo := strings.Split(partitionInfoLines[1], ":")
	// If EFI partition is used, use the second partition as it is root partition
	if firstPartitionInfo[4] == "fat16" {
		firstPartitionInfo = strings.Split(partitionInfoLines[2], ":")
	}
	firstPartitionEndInBytes, err := strconv.ParseUint(strings.TrimRight(firstPartitionInfo[2], "B"), 10, 64)
	if err != nil {
		return 0, bosherr.WrapErrorf(err, "Getting remaining size of `%s'", devicePath)
	}

	remainingSizeInBytes := deviceFullSizeInBytes - firstPartitionEndInBytes - 1

	return remainingSizeInBytes, nil
}

func (p rootDevicePartitioner) GetPartitions(devicePath string) (
	partitions []ExistingPartition,
	deviceFullSizeInBytes uint64,
	err error,
) {
	stdout, _, _, err := p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")
	if err != nil {
		return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Running parted print on `%s'", devicePath)
	}

	p.logger.Debug(p.logTag, "Found partitions %s", stdout)

	allLines := strings.Split(stdout, "\n")
	if len(allLines) < 3 {
		return partitions, deviceFullSizeInBytes, bosherr.Errorf("Parsing existing partitions of `%s'", devicePath)
	}

	partitionInfoLines := allLines[1:3]
	deviceInfo := strings.Split(partitionInfoLines[0], ":")
	deviceFullSizeInBytes, err = strconv.ParseUint(strings.TrimRight(deviceInfo[1], "B"), 10, 64)
	if err != nil {
		return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing device size `%s'", deviceInfo[1])
	}

	partitionLines := allLines[2 : len(allLines)-1]

	for _, partitionLine := range partitionLines {
		// ignore PReP partition on ppc64le
		if strings.Contains(partitionLine, "prep") {
			continue
		}
		partitionInfo := strings.Split(partitionLine, ":")
		partitionIndex, err := strconv.Atoi(partitionInfo[0])
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitionStartInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[1], "B"))
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitionEndInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[2], "B"))
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions of `%s'", devicePath)
		}

		partitionSizeInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[3], "B"))
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions of `%s'", devicePath)
		}

		// Saving partition type to detect if we're using EFI partition
		partitionType := PartitionTypeUnknown
		if partitionInfo[4] == "ext4" || partitionInfo[4] == "xfs" { //nolint:staticcheck
			partitionType = PartitionTypeLinux
		} else if partitionInfo[4] == "linux-swap(v1)" {
			partitionType = PartitionTypeSwap
		} else if partitionInfo[4] == "fat16" {
			partitionType = PartitionTypeEFI
		}

		partitions = append(
			partitions,
			ExistingPartition{
				Index:        partitionIndex,
				SizeInBytes:  uint64(partitionSizeInBytes),
				StartInBytes: uint64(partitionStartInBytes),
				EndInBytes:   uint64(partitionEndInBytes),
				Type:         partitionType,
			},
		)
	}
	return partitions, deviceFullSizeInBytes, nil
}

func (p rootDevicePartitioner) RemovePartitions(partitions []ExistingPartition, devicePath string) error {
	panic("unimplemented")
}

func (p rootDevicePartitioner) partitionsMatch(existingPartitions []ExistingPartition, partitions []Partition) bool {
	if len(existingPartitions) != len(partitions) {
		return false
	}

	for index, partition := range partitions {
		existingPartition := existingPartitions[index]

		if !withinDelta(partition.SizeInBytes, existingPartition.SizeInBytes, p.deltaInBytes) {
			return false
		}
	}

	return true
}

func (p rootDevicePartitioner) roundUp(numToRound, multiple uint64) uint64 {
	if multiple == 0 {
		return numToRound
	}
	remainder := numToRound % multiple
	if remainder == 0 {
		return numToRound
	}
	return numToRound + multiple - remainder
}

func (p rootDevicePartitioner) SinglePartitionNeedsResize(devicePath string, expectedPartitionType PartitionType) (needsResize bool, err error) {
	return false, bosherr.WrapError(err, "Resizing the system disk is not supported")
}

func (p rootDevicePartitioner) ResizeSinglePartition(devicePath string) (err error) {
	return bosherr.WrapError(err, "Resizing the system disk is not supported")
}
