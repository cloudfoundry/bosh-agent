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
		diskNumber string
		diskLetter string
	)

	BeforeEach(func() {
		if OsVersion == "2012R2" {
			Skip("Currently not supporting ephemeral disks on 2012R2")
		}

		diskNumber = "0"
		diskLetter = ""
	})

	AfterEach(func() {
		if agent != nil {
			agent.EnsureAgentServiceStopped()
			agent.EnsureDataDirDoesntExist()

			if diskLetter != "" {
				agent.RunPowershellCommand(fmt.Sprintf("Remove-Partition -DriveLetter %s -Confirm:$false", diskLetter))
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

		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when root disk partition is already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()
		agent.ShrinkRootPartition()
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)

		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))
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
		Expect(agent.PartitionCount("0")).To(Equal(1))

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
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when a second disk is attached and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))
	})

	It("when a second disk is attached and identified by index, partition is created on that disk", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-digit-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when a second disk is attached, identified by index and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-digit-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "1"
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))
	})

	It("when a third disk is attached, partition is created on that disk", func() {
		diskNumber = "2"

		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\third-disk-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when a third disk is attached and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\third-disk-settings.json c:\\bosh\\settings.json")

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		diskNumber = "2"
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.AgentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		agent.EnsureLinkTargettedToDisk(dataDir, diskNumber)
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))
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
		Expect(agent.PartitionCount("0")).To(Equal(1))

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
