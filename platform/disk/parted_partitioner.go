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

const (
	partitionNamePrefix = "bosh-partition"
	deltaSize           = 100
)

type partedPartitioner struct {
	logger      boshlog.Logger
	cmdRunner   boshsys.CmdRunner
	logTag      string
	timeService clock.Clock
}

func NewPartedPartitioner(logger boshlog.Logger, cmdRunner boshsys.CmdRunner, timeService clock.Clock) Partitioner {
	return partedPartitioner{
		logger:      logger,
		cmdRunner:   cmdRunner,
		logTag:      "PartedPartitioner",
		timeService: timeService,
	}
}

func (p partedPartitioner) Partition(devicePath string, desiredPartitions []Partition) error {
	_, _, _, err := p.cmdRunner.RunCommand("partprobe", devicePath)
	if err != nil {
		p.logger.Error(p.logTag, "Failed to probe existing parition: %s", err)
		return bosherr.WrapErrorf(err, "Re-reading partition table for `%s'", devicePath)
	}

	existingPartitions, deviceFullSizeInBytes, err := p.GetPartitions(devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting existing partitions of `%s'", devicePath)
	}

	if p.partitionsMatch(existingPartitions, desiredPartitions, deviceFullSizeInBytes) {
		return nil
	}

	if p.areAnyExistingPartitionsCreatedByBosh(existingPartitions) {
		return bosherr.Errorf("'%s' contains a partition created by bosh. No partitioning is allowed.", devicePath)
	}

	if err = p.createEachPartition(desiredPartitions, deviceFullSizeInBytes, devicePath); err != nil {
		return err
	}

	if strings.Contains(devicePath, "/dev/mapper/") {
		if err = p.createMapperPartition(devicePath); err != nil {
			return err
		}
	}

	return nil
}

func (p partedPartitioner) GetDeviceSizeInBytes(devicePath string) (uint64, error) {
	stdout, _, _, err := p.cmdRunner.RunCommand("lsblk", "--nodeps", "-nb", "-o", "SIZE", devicePath)
	if err != nil {
		return 0, bosherr.WrapErrorf(err, "Getting block device size of '%s'", devicePath)
	}

	deviceSize, err := strconv.Atoi(strings.Trim(stdout, "\n"))
	if err != nil {
		return 0, bosherr.WrapErrorf(err, "Converting block device size of '%s'", devicePath)
	}

	return uint64(deviceSize), nil
}

func (p partedPartitioner) partitionsMatch(existingPartitions []ExistingPartition, desiredPartitions []Partition, deviceSizeInBytes uint64) bool {
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
		} else if !withinDelta(partition.SizeInBytes, existingPartition.SizeInBytes, ConvertFromMbToBytes(deltaSize)) {
			return false
		}

		remainingDiskSpace -= partition.SizeInBytes
	}

	return true
}

func (p partedPartitioner) areAnyExistingPartitionsCreatedByBosh(existingPartitions []ExistingPartition) bool {
	for _, partition := range existingPartitions {
		if strings.HasPrefix(partition.Name, partitionNamePrefix) {
			return true
		}
	}

	return false
}

// For reference on format of outputs: http://lists.alioth.debian.org/pipermail/parted-devel/2006-December/000573.html
func (p partedPartitioner) GetPartitions(devicePath string) (partitions []ExistingPartition, deviceFullSizeInBytes uint64, err error) {
	stdout, _, _, err := p.runPartedPrint(devicePath)
	if err != nil {
		return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Running parted print")
	}

	allLines := strings.Split(stdout, "\n")
	if len(allLines) < 2 {
		return partitions, deviceFullSizeInBytes, bosherr.Errorf("Parsing existing partitions")
	}

	deviceInfo := strings.Split(allLines[1], ":")
	deviceFullSizeInBytes, err = strconv.ParseUint(strings.TrimRight(deviceInfo[1], "B"), 10, 64)
	if err != nil {
		return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing device size")
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
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions")
		}

		partitionStartInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[1], "B"))
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions")
		}

		partitionEndInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[2], "B"))
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions")
		}

		partitionSizeInBytes, err := strconv.Atoi(strings.TrimRight(partitionInfo[3], "B"))
		if err != nil {
			return partitions, deviceFullSizeInBytes, bosherr.WrapErrorf(err, "Parsing existing partitions")
		}

		partitionType := PartitionTypeUnknown
		if partitionInfo[4] == "ext4" || partitionInfo[4] == "xfs" { //nolint:staticcheck
			partitionType = PartitionTypeLinux
		} else if partitionInfo[4] == "linux-swap(v1)" {
			partitionType = PartitionTypeSwap
		}

		partitionName := partitionInfo[5]

		partitions = append(
			partitions,
			ExistingPartition{
				Index:        partitionIndex,
				SizeInBytes:  uint64(partitionSizeInBytes),
				StartInBytes: uint64(partitionStartInBytes),
				EndInBytes:   uint64(partitionEndInBytes),
				Type:         partitionType,
				Name:         partitionName,
			},
		)
	}

	return partitions, deviceFullSizeInBytes, nil
}

func (p partedPartitioner) RemovePartitions(partitions []ExistingPartition, devicePath string) error {
	partitionRetryable := boshretry.NewRetryable(func() (bool, error) {
		_, _, _, err := p.cmdRunner.RunCommand(
			"wipefs",
			"--force", // to prevent "wipefs: error: /**: probing initialization failed: Device or resource busy" errors
			"-a",
			devicePath,
		)
		if err != nil {
			return true, bosherr.WrapError(err, fmt.Sprintf("Removing device path `%s' ", devicePath))
		}

		p.logger.Info(p.logTag, "Successfully removed device path `%s'", devicePath)
		return false, nil
	})

	partitionRetryStrategy := NewPartitionStrategy(partitionRetryable, p.timeService, p.logger)
	err := partitionRetryStrategy.Try()
	if err != nil {
		return err
	}

	return nil
}

func (p partedPartitioner) runPartedPrint(devicePath string) (stdout, stderr string, exitStatus int, err error) {
	stdout, stderr, exitStatus, err = p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")

	defer p.cmdRunner.RunCommand("udevadm", "settle") // nolint:errcheck

	printFields := strings.SplitN(stdout, ":", 7)

	// Create a new partition table if
	// - there is none, or
	// - a "loop" partition table is shown (which can mean a valid one was not found)
	containsUnrecognizedDiskLabel := strings.Contains(fmt.Sprintf("%s\n%s", stdout, stderr), "unrecognised disk label")
	if containsUnrecognizedDiskLabel || (len(printFields) > 5 && printFields[5] == "loop") {
		stdout, stderr, exitStatus, err = p.getPartitionTable(devicePath)
		if err != nil {
			return stdout, stderr, exitStatus, bosherr.WrapErrorf(err, "Parted making label")
		}

		return p.cmdRunner.RunCommand("parted", "-m", devicePath, "unit", "B", "print")
	}

	return stdout, stderr, exitStatus, err
}

func (p partedPartitioner) getPartitionTable(devicePath string) (stdout, stderr string, exitStatus int, err error) {
	return p.cmdRunner.RunCommand(
		"parted",
		"-s",
		devicePath,
		"mklabel",
		"gpt",
	)
}

func (p partedPartitioner) roundUp(numToRound, multiple uint64) uint64 {
	if multiple == 0 {
		return numToRound
	}
	remainder := numToRound % multiple
	if remainder == 0 {
		return numToRound
	}
	return numToRound + multiple - remainder
}

func (p partedPartitioner) roundDown(numToRound, multiple uint64) uint64 {
	if multiple == 0 {
		return numToRound
	}
	remainder := numToRound % multiple
	if remainder == 0 {
		return numToRound
	}
	return numToRound - remainder
}

func (p partedPartitioner) createEachPartition(partitions []Partition, deviceFullSizeInBytes uint64, devicePath string) error {
	partitionStart := uint64(1048576)
	alignmentInBytes := uint64(1048576)

	for index, partition := range partitions {
		var partitionEnd uint64

		if partition.SizeInBytes == 0 {
			partitionEnd = deviceFullSizeInBytes - 1
		} else {
			partitionEnd = partitionStart + partition.SizeInBytes
			if partitionEnd >= deviceFullSizeInBytes {
				partitionEnd = deviceFullSizeInBytes - 1
				p.logger.Info(p.logTag, "Partition %d would be larger than remaining space. Reducing size to %dB", index, partitionEnd-partitionStart)
			}
		}
		partitionEnd = p.roundDown(partitionEnd, alignmentInBytes) - 1

		if len(partition.NamePrefix) == 0 {
			partition.NamePrefix = partitionNamePrefix
		}

		partitionRetryable := boshretry.NewRetryable(func() (bool, error) {
			_, _, _, err := p.cmdRunner.RunCommand(
				"parted",
				"-s",
				devicePath,
				"unit",
				"B",
				"mkpart",
				fmt.Sprintf("%s-%d", partition.NamePrefix, index),
				fmt.Sprintf("%d", partitionStart),
				fmt.Sprintf("%d", partitionEnd),
			)
			if err != nil {
				p.logger.Error(p.logTag, "Failed with an error: %s", err)
				// TODO: double check the output here. Does it make sense?
				return true, bosherr.WrapError(err, "Creating partition using parted")
			}

			_, _, _, err = p.cmdRunner.RunCommand("partprobe", devicePath)
			if err != nil {
				p.logger.Error(p.logTag, "Failed to probe for newly created parition: %s", err)
				return true, bosherr.WrapError(err, "Creating partition using parted")
			}

			_, _, _, err = p.cmdRunner.RunCommand("udevadm", "settle")
			if err != nil {
				p.logger.Error(p.logTag, "Failed to run udevadm settle: %s", err)
			}

			p.logger.Info(p.logTag, "Successfully created partition %d on %s", index, devicePath)
			return false, nil
		})

		partitionRetryStrategy := NewPartitionStrategy(partitionRetryable, p.timeService, p.logger)
		err := partitionRetryStrategy.Try()

		if err != nil {
			return bosherr.WrapErrorf(err, "Partitioning disk `%s'", devicePath)
		}

		partitionStart = p.roundUp(partitionEnd+1, alignmentInBytes)
	}
	return nil
}

func (p partedPartitioner) createMapperPartition(devicePath string) error {
	_, _, _, err := p.cmdRunner.RunCommand("/etc/init.d/open-iscsi", "restart")
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to restart open-iscsi")
	}

	_, _, _, err = p.cmdRunner.RunCommand("/etc/init.d/multipath-tools", "restart")
	if err != nil {
		return bosherr.WrapError(err, "Restarting multipath after restarting open-iscsi")
	}

	detectPartitionRetryable := boshretry.NewRetryable(func() (bool, error) {
		output, _, _, err := p.cmdRunner.RunCommand("dmsetup", "ls")
		if err != nil {
			return true, bosherr.WrapError(err, "Shelling out to dmsetup ls")
		}

		if strings.Contains(output, "No devices found") {
			return true, bosherr.Errorf("No devices found")
		}

		part1Regexp := regexp.MustCompile("-part1")
		device := strings.TrimPrefix(devicePath, "/dev/mapper/")
		lines := strings.Split(strings.Trim(output, "\n"), "\n")
		for i := 0; i < len(lines); i++ {
			if part1Regexp.MatchString(lines[i]) {
				if strings.Contains(lines[i], device) {
					p.logger.Info(p.logTag, "Succeeded in detecting partition %s", devicePath+"-part1")
					return false, nil
				}
			}
		}

		return true, bosherr.Errorf("Partition %s does not show up", devicePath+"-part1")
	})

	detectPartitionRetryStrategy := NewPartitionStrategy(detectPartitionRetryable, p.timeService, p.logger)
	return detectPartitionRetryStrategy.Try()
}

func (p partedPartitioner) SinglePartitionNeedsResize(devicePath string, expectedPartitionType PartitionType) (needsResize bool, err error) {
	existingPartitions, diskSize, err := p.GetPartitions(devicePath)
	if err != nil {
		return false, bosherr.WrapError(err, "Failed to get existing partitions")
	}
	if len(existingPartitions) > 1 {
		return false, bosherr.Errorf(
			"Persistent disks with many partitions are not supported. Expected 1, got %d.",
			len(existingPartitions))
	}
	if len(existingPartitions) < 1 {
		return false, nil
	}

	partition := existingPartitions[0]
	if partition.Type != expectedPartitionType {
		return false, nil
	}
	return significantlySmallerThan(
		partition.SizeInBytes, diskSize, ConvertFromMbToBytes(deltaSize)), nil
}

func (p partedPartitioner) ResizeSinglePartition(devicePath string) (err error) {
	if !p.cmdRunner.CommandExists("growpart") {
		p.logger.Info(p.logTag, "The program 'growpart' is not installed, Persistent Filesystem cannot be grown")
		return bosherr.Error("The program 'growpart' is not installed, Persistent Filesystem cannot be grown")
	}

	if !p.cmdRunner.CommandExists("partx") {
		p.logger.Info(p.logTag, "The program 'partx' is not installed, Persistent Filesystem cannot be grown")
		return bosherr.Error("The program 'partx' is not installed, Persistent Filesystem cannot be grown")
	}

	partitionRetryable := boshretry.NewRetryable(func() (bool, error) {
		_, _, _, err := p.cmdRunner.RunCommand(
			"growpart", devicePath, "1", "--update", "auto",
		)
		if err != nil {
			p.logger.Error(p.logTag, "Failed with an error: %s", err)
			return true, bosherr.WrapError(err, "Resizing partition using growpart")
		}

		p.logger.Info(p.logTag, "Successfully resized sinlge partition in %s", devicePath)
		return false, nil
	})

	partitionRetryStrategy := NewPartitionStrategy(partitionRetryable, p.timeService, p.logger)
	err = partitionRetryStrategy.Try()

	if err != nil {
		return bosherr.WrapErrorf(err, "Repartitioning disk `%s'", devicePath)
	}

	return nil
}
