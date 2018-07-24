package windows_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EphemeralDisk", func() {
	var (
		partitionNumber string
		diskNumber      string
	)

	BeforeEach(func() {
		partitionNumber = ""
		diskNumber = "0"
	})

	AfterEach(func() {
		if agent != nil {
			agent.EnsureAgentServiceStopped()
			agent.EnsureDataDirDoesntExist()

			if partitionNumber != "" {
				agent.RunPowershellCommand(
					fmt.Sprintf(
						"Remove-Partition -DiskNumber %s -PartitionNumber %s -Confirm:$false",
						diskNumber,
						partitionNumber,
					),
				)
			}
			if diskNumber != "0" {
				agent.EnsureDiskCleared(diskNumber)
			}
			agent.EnsureRootPartitionAtMaxSize()

			agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\agent.json c:\\bosh\\agent.json")
			agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-disk-settings.json c:\\bosh\\settings.json")
			agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")
		}
	})

	It("when root disk can be used as ephemeral, creates a partition on root disk", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()
		agent.ShrinkRootPartition()
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		agent.EnsureVolumeHasDataDir("0")
		partitionNumber = agent.GetDataDirPartitionNumber()

		agent.AssertDataACLed()
	})

	It("when root disk partition is already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()
		agent.ShrinkRootPartition()
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		agent.EnsureVolumeHasDataDir("0")

		partitionNumber = agent.GetDataDirPartitionNumber()

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
	})

	It("when there is no remaining space on the root disk, no partition is created, a warning is logged", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		Expect(agent.PartitionWithDataDirExists("0")).To(BeFalse())

		expectedLogMessage := fmt.Sprintf(
			"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
		)
		matchingLogOutput := agent.RunPowershellCommand(fmt.Sprintf(
			`Select-String -Path C:\var\vcap\bosh\log\service_wrapper.err.log -Pattern "%s"`,
			expectedLogMessage,
		))
		Expect(strings.TrimSpace(matchingLogOutput)).NotTo(BeEmpty())
	})

	It("when a second disk is attached, partition is created on that disk", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureVolumeHasDataDir("1")
		partitionNumber = agent.GetDataDirPartitionNumber()

		agent.AssertDataACLed()
	})

	It("when a second disk is attached and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureVolumeHasDataDir("1")
		partitionNumber = agent.GetDataDirPartitionNumber()

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		agent.EnsureVolumeHasDataDir("1")
	})

	It("when a second disk is attached and identified by index, partition is created on that disk", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-digit-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureVolumeHasDataDir("1")
		partitionNumber = agent.GetDataDirPartitionNumber()

		agent.AssertDataACLed()
	})

	It("when a second disk is attached, identified by index and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-digit-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureVolumeHasDataDir("1")
		partitionNumber = agent.GetDataDirPartitionNumber()

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		agent.EnsureVolumeHasDataDir("1")
	})

	It("when the EphemeralDiskFeature flag is not set doesn't create any partitions, or send any warnings", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()
		agent.ShrinkRootPartition()
		agent.RunPowershellCommand(
			"cp c:\\bosh\\agent-configuration\\root-partition-agent-ephemeral-disabled.json c:\\bosh\\agent.json",
		)
		agent.RunPowershellCommand(`Remove-Item C:\var\vcap\bosh\log\service_wrapper.err.log`)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		Expect(agent.PartitionWithDataDirExists("0")).To(BeFalse())

		unexpectedLogMessage := fmt.Sprintf(
			"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
		)
		matchingLogOutput := agent.RunPowershellCommand(fmt.Sprintf(
			`Select-String -Path C:\var\vcap\bosh\log\service_wrapper.err.log -Pattern "%s"`,
			unexpectedLogMessage,
		))
		Expect(strings.TrimSpace(matchingLogOutput)).To(BeEmpty())
	})
})
