package disk

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type EphemeralDevicePartitioner struct {
	partedPartitioner Partitioner
	logger            boshlog.Logger

	logTag    string
	cmdRunner boshsys.CmdRunner
}

type Settings struct {
	AgentID string `json:"agent_id"`
}

func NewEphemeralDevicePartitioner(
	partedPartitioner Partitioner,
	logger boshlog.Logger,
	cmdRunner boshsys.CmdRunner,
) *EphemeralDevicePartitioner {
	return &EphemeralDevicePartitioner{
		partedPartitioner: partedPartitioner,
		logger:            logger,
		logTag:            "EphemeralDevicePartitioner",
		cmdRunner:         cmdRunner,
	}
}

func (p *EphemeralDevicePartitioner) Partition(devicePath string, partitions []Partition) error {
	existingPartitions, deviceFullSizeInBytes, err := p.partedPartitioner.GetPartitions(devicePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Getting existing partitions of `%s'", devicePath)
	}

	if p.matchPartitionNames(existingPartitions, partitions, deviceFullSizeInBytes) {
		p.logger.Info(p.logTag, "%s already partitioned as expected, skipping", devicePath)
		return nil
	}

	err = p.RemovePartitions(existingPartitions, devicePath)
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

func (p *EphemeralDevicePartitioner) matchPartitionNames(existingPartitions []ExistingPartition, desiredPartitions []Partition, deviceSizeInBytes uint64) bool {
	if len(existingPartitions) < len(desiredPartitions) {
		return false
	}

	for index, partition := range desiredPartitions {
		existingPartition := existingPartitions[index]

		if !strings.HasPrefix(existingPartition.Name, partition.NamePrefix) {
			return false
		}

	}

	return true
}

func (p EphemeralDevicePartitioner) RemovePartitions(partitions []ExistingPartition, devicePath string) error {
	return p.partedPartitioner.RemovePartitions(partitions, devicePath)
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
