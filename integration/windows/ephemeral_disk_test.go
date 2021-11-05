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
		diskNumber = "0"
		diskLetter = ""
	})

	AfterEach(func() {
		agent.EnsureAgentServiceStopped()
		agent.EnsureDataDirDoesntExist()

		if diskLetter != "" {
			agent.RunPowershellCommand(fmt.Sprintf("Remove-Partition -DriveLetter %s -Confirm:$false", diskLetter))
		}
		if diskNumber != "0" {
			agent.EnsureDiskCleared(diskNumber)
		}
		agent.EnsureRootPartitionAtMaxSize()
	})

	It("when root disk can be used as ephemeral, creates a partition on root disk", func() {
		agent.EnsureDataDirDoesntExist()
		agent.ShrinkRootPartition()
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-disk-settings.json c:\\bosh\\settings.json")
		agent.StartAgent()

		agent.EnsureLinkTargetedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when root disk partition is already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureDataDirDoesntExist()
		agent.ShrinkRootPartition()
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.StartAgent()

		agent.EnsureLinkTargetedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Eventually(func() bool {
			return agent.IsLinkTargetedToDisk(dataDir, diskNumber)
		}, 60*time.Second).Should(BeTrue())
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))

		Expect(agent.AgentProcessRunning()).To(BeTrue())
	})

	It("when there is no remaining space on the root disk, no partition is created, a warning is logged", func() {
		initialPartitionCount := agent.PartitionCount("0")
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.StartAgent()

		expectedLogMessage := fmt.Sprintf(
			"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
		)
		Eventually(func() bool {
			matchingLogOutput := agent.RunPowershellCommand(fmt.Sprintf(
				`Select-String -Path C:\var\vcap\bosh\log\service_wrapper.err.log -Pattern "%s"`,
				expectedLogMessage,
			))
			return len(strings.TrimSpace(matchingLogOutput)) > 0
		}, 1*time.Minute, 5*time.Second).Should(
			BeTrue(),
			"Expected agent to warn about full disk",
		)

		Expect(agent.PartitionCount("0")).To(Equal(initialPartitionCount))
		Expect(agent.AgentProcessRunning()).To(BeTrue())
	})

	It("when a second disk is attached, partition is created on that disk", func() {
		agent.EnsureDataDirDoesntExist()

		diskNumber = "1"
		agent.EnsureDiskCleared(diskNumber)

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-settings.json c:\\bosh\\settings.json")

		agent.StartAgent()

		Eventually(func() bool {
			return agent.IsLinkTargetedToDisk(dataDir, diskNumber)
		}, 60*time.Second).Should(BeTrue())
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when a second disk is attached and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-settings.json c:\\bosh\\settings.json")

		agent.StartAgent()

		diskNumber = "1"
		agent.EnsureLinkTargetedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Eventually(func() bool {
			return agent.IsLinkTargetedToDisk(dataDir, diskNumber)
		}, 60*time.Second).Should(BeTrue())
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))

		Expect(agent.AgentProcessRunning()).To(BeTrue())
	})

	It("when a second disk is attached and identified by index, partition is created on that disk", func() {
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-digit-settings.json c:\\bosh\\settings.json")

		agent.StartAgent()

		diskNumber = "1"
		agent.EnsureLinkTargetedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.AssertDataACLed()
	})

	It("when a second disk is attached, identified by index and already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.EnsureDataDirDoesntExist()

		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\second-disk-digit-settings.json c:\\bosh\\settings.json")

		agent.StartAgent()

		diskNumber = "1"
		agent.EnsureLinkTargetedToDisk(dataDir, diskNumber)
		diskLetter = agent.GetDriveLetterForLink(dataDir)

		agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Eventually(func() bool {
			return agent.IsLinkTargetedToDisk(dataDir, diskNumber)
		}, 60*time.Second).Should(BeTrue())
		Expect(agent.GetDriveLetterForLink(dataDir)).To(Equal(diskLetter))

		Expect(agent.AgentProcessRunning()).To(BeTrue())
	})
})
