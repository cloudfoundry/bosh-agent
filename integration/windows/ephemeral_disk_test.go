package windows_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"strconv"

	"github.com/masterzen/winrm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EphemeralDisk", func() {
	const dataDir = `C:\var\vcap\data\`

	var (
		partitionNumber string
		agent           *windowsEnvironment
	)

	BeforeEach(func() {
		if OsVersion != "2012R2" {
			Skip("Ephemeral disk mounting only configured for 2012R2")
		}

		endpoint := winrm.NewEndpoint(os.Getenv("AGENT_ELASTIC_IP"), 5985, false, false, nil, nil, nil, 0)
		client, err := winrm.NewClient(endpoint, "vagrant", "Password123!")
		Expect(err).NotTo(HaveOccurred())
		partitionNumber = ""

		agent = &windowsEnvironment{
			dataDir: dataDir,
			client:  client,
		}
	})

	AfterEach(func() {
		if agent != nil {
			agent.ensureAgentServiceStopped()
			agent.ensureDataDirDoesntExist()

			if partitionNumber != "" {
				agent.runPowershellCommand(
					fmt.Sprintf("Remove-Partition -DiskNumber 0 -PartitionNumber %s -Confirm:$false", partitionNumber),
				)
			}
			agent.ensureRootPartitionAtMaxSize()

			agent.runPowershellCommand("cp c:\\bosh\\agent-configuration\\agent.json c:\\bosh\\agent.json")
			agent.runPowershellCommand("c:\\bosh\\service_wrapper.exe start")
		}
	})

	It("when root disk can be used as ephemeral, creates a partition on root disk", func() {
		agent.ensureAgentServiceStopped()
		agent.ensureDataDirDoesntExist()
		agent.shrinkRootPartition()
		agent.runPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.runPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		agent.ensureVolumeHasDataDir("0")

		partitionNumber = agent.getDataDirPartitionNumber()

	})

	It("when root disk partition is already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		agent.ensureAgentServiceStopped()
		agent.ensureDataDirDoesntExist()
		agent.shrinkRootPartition()
		agent.runPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		agent.runPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		agent.ensureVolumeHasDataDir("0")

		partitionNumber = agent.getDataDirPartitionNumber()

		agent.runPowershellCommand("c:\\bosh\\service_wrapper.exe restart")

		Consistently(agent.agentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
	})

	It("when there is no remaining space on the root disk, no partititon is created, a warning is logged", func() {
		agent.ensureAgentServiceStopped()
		agent.ensureDataDirDoesntExist()

		agent.runPowershellCommand("cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")
		agent.runPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		Consistently(agent.agentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		Expect(agent.partitionWithDataDirExists("0")).To(BeFalse())

		expectedLogMessage := fmt.Sprintf(
			"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
		)
		matchingLogOutput := agent.runPowershellCommand(fmt.Sprintf(
			`Select-String -Path C:\var\vcap\bosh\log\service_wrapper.err.log -Pattern "%s"`,
			expectedLogMessage,
		))
		Expect(strings.TrimSpace(matchingLogOutput)).NotTo(BeEmpty())
	})

	It("when the EphemeralDiskFeature flag is not set doesn't create any partitions, or send any warnings", func() {
		agent.ensureAgentServiceStopped()
		agent.ensureDataDirDoesntExist()
		agent.shrinkRootPartition()
		agent.runPowershellCommand(
			"cp c:\\bosh\\agent-configuration\\root-partition-agent-ephemeral-disabled.json c:\\bosh\\agent.json",
		)
		agent.runPowershellCommand(`Remove-Item C:\var\vcap\bosh\log\service_wrapper.err.log`)

		agent.runPowershellCommand("c:\\bosh\\service_wrapper.exe start")

		Consistently(agent.agentProcessRunningFunc(), 60*time.Second).Should(
			BeTrue(),
			fmt.Sprint(`Expected bosh-agent to continue running after restart`),
		)
		Expect(agent.partitionWithDataDirExists("0")).To(BeFalse())

		unexpectedLogMessage := fmt.Sprintf(
			"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
		)
		matchingLogOutput := agent.runPowershellCommand(fmt.Sprintf(
			`Select-String -Path C:\var\vcap\bosh\log\service_wrapper.err.log -Pattern "%s"`,
			unexpectedLogMessage,
		))
		Expect(strings.TrimSpace(matchingLogOutput)).To(BeEmpty())
	})
})

type windowsEnvironment struct {
	client  *winrm.Client
	dataDir string
}

func (e *windowsEnvironment) shrinkRootPartition() {
	e.runPowershellCommandWithOffset(
		1,
		"Get-Partition -DriveLetter C | Resize-Partition -Size $(Get-PartitionSupportedSize -DriveLetter C).SizeMin",
	)
}

func (e *windowsEnvironment) ensureRootPartitionAtMaxSize() {
	freeSpaceOutput := e.runPowershellCommandWithOffset(
		1,
		"Get-Disk $(Get-Partition -DriveLetter C | Select -ExpandProperty DiskNumber) | Select -ExpandProperty LargestFreeExtent",
	)

	freeSpace, err := strconv.Atoi(strings.TrimSpace(freeSpaceOutput))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	if freeSpace > 0 {
		e.runPowershellCommandWithOffset(
			1,
			"Resize-Partition -DriveLetter C -Size $(Get-PartitionSupportedSize -DriveLetter C).SizeMax",
		)
	}
}

func (e *windowsEnvironment) getDataDirPartitionNumber() string {
	return strings.TrimSpace(e.runPowershellCommandWithOffset(
		1,
		fmt.Sprintf(`Get-Partition | Where AccessPaths -Contains "%s" | Select -ExpandProperty PartitionNumber`, e.dataDir),
	))
}

func (e *windowsEnvironment) partitionWithDataDirExists(diskNumber string) bool {
	return e.partitionWithDataDirExistsWithOffset(1, diskNumber)
}

func (e *windowsEnvironment) partitionWithDataDirExistsWithOffset(offset int, diskNumber string) bool {
	stdout := e.runPowershellCommandWithOffset(
		offset+1,
		fmt.Sprintf(
			`Get-Partition | where AccessPaths -Contains "%s" | Select -ExpandProperty DiskNumber`,
			e.dataDir,
		),
	)

	return strings.TrimSpace(stdout) == diskNumber
}

func (e *windowsEnvironment) partitionWithDataDirExistsFuncWithOffset(offset int, diskNumber string) func() bool {
	return func() bool {
		return e.partitionWithDataDirExistsWithOffset(offset+1, diskNumber)
	}
}

func (e *windowsEnvironment) partitionWithDataDirExistsFunc(diskNumber string) func() bool {
	return e.partitionWithDataDirExistsFuncWithOffset(1, diskNumber)
}

func (e *windowsEnvironment) ensureVolumeHasDataDir(diskNumber string) {
	EventuallyWithOffset(1, e.partitionWithDataDirExistsFunc(diskNumber), 60*time.Second).Should(
		BeTrue(),
		fmt.Sprintf(`Expected partition with access path %s to be present on disk %s`, e.dataDir, diskNumber),
	)
}

func (e *windowsEnvironment) ensureAgentServiceStopped() {
	stdout := e.runPowershellCommandWithOffset(1, "Get-Service -Name bosh-agent | Format-List -Property Status")

	running, err := regexp.MatchString("Running", stdout)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	if running {
		e.runPowershellCommandWithOffset(1, "c:\\bosh\\service_wrapper.exe stop")
	}
}

func (e *windowsEnvironment) ensureDataDirDoesntExist() {
	testPathOutput := e.runPowershellCommand("Test-Path -Path %s", e.dataDir)

	exists := strings.TrimSpace(testPathOutput) == "True"
	if exists {
		e.runPowershellCommandWithOffset(1, "Remove-Item %s -Force -Recurse", e.dataDir)
	}
}

func (e *windowsEnvironment) agentProcessRunningFunc() func() bool {
	return func() bool {
		exitCode, err := e.client.Run(
			winrm.Powershell("Get-Process -ProcessName bosh-agent"),
			ioutil.Discard, ioutil.Discard,
		)
		return exitCode == 0 && err == nil
	}
}

func (e *windowsEnvironment) runPowershellCommandWithOffset(offset int, cmd string, cmdFmtArgs ...interface{}) string {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := e.client.Run(winrm.Powershell(fmt.Sprintf(cmd, cmdFmtArgs...)), stdout, stderr)

	outString := stdout.String()
	errString := stderr.String()

	ExpectWithOffset(offset+1, err).NotTo(
		HaveOccurred(),
		fmt.Sprintf(`Command "%s" failed with stdout: %s; stderr: %s`, cmd, outString, errString),
	)
	ExpectWithOffset(offset+1, exitCode).To(
		BeZero(),
		fmt.Sprintf(
			`Command "%s" failed with exit code: %d; stdout: %s; stderr: %s`,
			cmd,
			exitCode,
			stdout.String(),
			stderr.String(),
		),
	)

	return outString
}

func (e *windowsEnvironment) runPowershellCommand(cmd string, cmdFmtArgs ...interface{}) string {
	return e.runPowershellCommandWithOffset(1, cmd, cmdFmtArgs...)
}
