package windows_test

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
)

const dataDir = `C:\var\vcap\data\`

const GB = 1024 * 1024 * 1024

type WindowsEnvironment struct {
	Client *winrm.Client
	Linker *disk.Linker
}

func (e *WindowsEnvironment) ShrinkRootPartition() {
	retryableFailure := "Resize-Partition : Size Not Supported"
	retryableError := "net/http: timeout awaiting response headers"
	retryableSizeMin := "Resize-Partition : Not enough available capacity"

	sizeMinBuffer := 1 * GB
	cmdFmtString := "Get-Partition -DriveLetter C | Resize-Partition -Size $((Get-PartitionSupportedSize -DriveLetter C).SizeMin + %d)"

	for i := 0; i < 5; i++ {
		cmd := fmt.Sprintf(cmdFmtString, sizeMinBuffer)
		stdout, stderr, exitCode, err := e.RunPowershellCommandWithOffsetAndResponses(
			1,
			cmd,
		)

		if err != nil {
			if strings.Contains(err.Error(), retryableError) {
				fmt.Printf("WinRM timed out on attempt %d of 5, waiting 5 seconds to retry command: %s\n", i+1, cmd)
				time.Sleep(5 * time.Second)
				continue
			} else {
				ExpectWithOffset(1, err).NotTo(
					HaveOccurred(),
					fmt.Sprintf(`Command "%s" failed with stdout: %s; stderr: %s`, cmd, stdout, stderr),
				)
			}
		}

		if exitCode != 0 {
			if strings.Contains(stderr, retryableFailure) {
				fmt.Printf("Failed to shrink disk on attempt %d of 5, waiting 5 seconds to retry\n", i+1)
				time.Sleep(5 * time.Second)
				continue
			} else if strings.Contains(stderr, retryableSizeMin) {
				fmt.Printf("Failed to shrink disk on attempt %d of 5, waiting 5 seconds to retry\n", i+1)
				time.Sleep(5 * time.Second)
				sizeMinBuffer += 2 * GB
				continue
			} else {
				ExpectWithOffset(1, exitCode).To(
					BeZero(),
					fmt.Sprintf(
						`Command "%s" failed with exit code: %d; stdout: %s; stderr: %s`, cmd, exitCode, stdout, stderr,
					),
				)
			}
		}

		break
	}
}

func (e *WindowsEnvironment) EnsureRootPartitionAtMaxSize() {
	freeSpaceOutput := e.RunPowershellCommandWithOffset(
		1,
		"Get-Disk $(Get-Partition -DriveLetter C | Select -ExpandProperty DiskNumber) | Select -ExpandProperty LargestFreeExtent",
	)

	freeSpace, err := strconv.Atoi(strings.TrimSpace(freeSpaceOutput))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	if freeSpace > 0 {
		e.RunPowershellCommandWithOffset(
			1,
			"Resize-Partition -DriveLetter C -Size $(Get-PartitionSupportedSize -DriveLetter C).SizeMax",
		)
	}
}

func (e *WindowsEnvironment) GetDriveLetterForLink(path string) string {
	return e.GetDriveLetterForLinkWithOffset(1, path)
}

func (e *WindowsEnvironment) GetDriveLetterForLinkWithOffset(offset int, path string) string {
	target, err := e.Linker.LinkTarget(path)

	ExpectWithOffset(offset+1, err).NotTo(HaveOccurred())
	return strings.Split(target, ":")[0]
}

func (e *WindowsEnvironment) CleanUpExtraDisks() {
	e.EnsureDataDirDoesntExist()
	partitions := e.PartitionCount("0")
	cDrivePartition, err := strconv.Atoi(strings.TrimSpace(e.RunPowershellCommand("(Get-Partition -DriveLetter C).PartitionNumber")))
	Expect(err).ToNot(HaveOccurred())

	for i := partitions; i > cDrivePartition; i-- {
		e.RunPowershellCommandWithOffset(
			1,
			fmt.Sprintf("Remove-Partition -DiskNumber 0 -PartitionNumber %d -Confirm:0", i),
		)
	}
}

func (e *WindowsEnvironment) GetDiskNumberForDrive(driveLetter string) string {
	return strings.TrimSpace(e.RunPowershellCommandWithOffset(
		1,
		fmt.Sprintf(`Get-Partition -DriveLetter %s | Select -ExpandProperty DiskNumber`, driveLetter),
	))
}

func (e *WindowsEnvironment) IsLinkTargetedToDisk(path, diskNumber string) bool {
	e.WaitForLinkWithOffset(1, path)

	diskLetter := e.GetDriveLetterForLinkWithOffset(1, path)
	actualDiskNumber := agent.GetDiskNumberForDrive(diskLetter)
	return actualDiskNumber == diskNumber
}

func (e *WindowsEnvironment) EnsureLinkTargetedToDisk(path, diskNumber string) {
	ExpectWithOffset(1, e.IsLinkTargetedToDisk(path, diskNumber)).To(BeTrue())
}

func (e *WindowsEnvironment) WaitForLink(path string) {
	e.WaitForLinkWithOffset(1, path)
}

func (e *WindowsEnvironment) WaitForLinkWithOffset(offset int, path string) {
	EventuallyWithOffset(offset+1, func() bool {
		target, _ := e.Linker.LinkTarget(path)
		return target != ""
	}, 2*time.Minute, 5*time.Second).Should(BeTrue())
}

func (e *WindowsEnvironment) CleanUpUpdateSettings() {
	agent.RunPowershellCommand("rm c:\\var\\vcap\\bosh\\update_settings.json")
}

func (e *WindowsEnvironment) StartAgent() {
	agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe install")
	agent.RunPowershellCommand("c:\\bosh\\service_wrapper.exe start")
}

func (e *WindowsEnvironment) EnsureAgentServiceStopped() {
	stdout := e.RunPowershellCommandWithOffset(1, "Get-Service -Name bosh-agent | Format-List -Property Status")

	running, err := regexp.MatchString("Running", stdout)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	if running {
		e.RunPowershellCommandWithOffset(1, "c:\\bosh\\service_wrapper.exe stop")
	}
	e.RunPowershellCommandWithOffset(1, "c:\\bosh\\service_wrapper.exe uninstall")
}

func (e *WindowsEnvironment) EnsureDataDirDoesntExist() {
	testPathOutput := e.RunPowershellCommandWithOffset(1, fmt.Sprintf("Test-Path -Path %s", dataDir))

	exists := strings.TrimSpace(testPathOutput) == "True"
	if exists {
		e.RunPowershellCommandWithOffset(1, fmt.Sprintf("cmd.exe /c rmdir /s /q %s", dataDir))
	}
}

func (e *WindowsEnvironment) AgentProcessRunning() bool {
	exitCode, err := e.Client.Run( //nolint:staticcheck
		winrm.Powershell("Get-Process -ProcessName bosh-agent"),
		io.Discard, io.Discard,
	)
	return exitCode == 0 && err == nil
}

func (e *WindowsEnvironment) RunPowershellCommandWithOffset(offset int, cmd string) string {
	outString, errString, exitCode, err := e.RunPowershellCommandWithOffsetAndResponses(offset+1, cmd)

	ExpectWithOffset(offset+1, err).NotTo(
		HaveOccurred(),
		fmt.Sprintf(`Command "%s" failed with stdout: %s; stderr: %s`, cmd, outString, errString),
	)
	ExpectWithOffset(offset+1, exitCode).To(
		BeZero(),
		fmt.Sprintf(
			`Command "%s" failed with exit code: %d; stdout: %s; stderr: %s`, cmd, exitCode, outString, errString,
		),
	)

	return outString
}

func (e *WindowsEnvironment) RunPowershellCommandWithOffsetAndResponses(_offset int, cmd string) (string, string, int, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := e.Client.Run(winrm.Powershell(cmd), stdout, stderr) //nolint:staticcheck

	outString := stdout.String()
	errString := stderr.String()

	return outString, errString, exitCode, err
}

func (e *WindowsEnvironment) RunPowershellCommandWithResponses(cmd string) (string, string, int, error) {
	return e.RunPowershellCommandWithOffsetAndResponses(1, cmd)
}

func (e *WindowsEnvironment) RunPowershellCommand(cmd string) string {
	return e.RunPowershellCommandWithOffset(1, cmd)
}

func (e *WindowsEnvironment) AssertDataACLed() {
	testFile := filepath.Join(dataDir + "testfile")

	e.RunPowershellCommandWithOffset(1, fmt.Sprintf("echo 'content' >> %s", testFile))
	checkACLsOutput := e.RunPowershellCommandWithOffset(1, fmt.Sprintf("Check-Acls %s", dataDir))
	aclErrsCount := strings.Count(checkACLsOutput, "Error")
	ExpectWithOffset(1, aclErrsCount == 0).To(
		BeTrue(),
		fmt.Sprintf("Expected data directory to have correct ACLs. Counted %d errors.", aclErrsCount),
	)
}

func (e *WindowsEnvironment) PartitionCount(diskNumber string) int {
	return e.PartitionCountWithOffset(1, diskNumber)
}

func (e *WindowsEnvironment) PartitionCountWithOffset(offset int, diskNumber string) int {
	partitionCountOutput := e.RunPowershellCommandWithOffset(
		offset+1,
		fmt.Sprintf("Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions", diskNumber),
	)

	partitionCount, err := strconv.Atoi(strings.TrimSpace(partitionCountOutput))
	ExpectWithOffset(offset+1, err).NotTo(HaveOccurred())

	return partitionCount
}

func (e *WindowsEnvironment) EnsureDiskClearedWithOffset(offset int, diskNumber string) {
	if e.PartitionCountWithOffset(offset+1, diskNumber) > 0 {
		e.RunPowershellCommandWithOffset(offset+1, fmt.Sprintf("Clear-Disk -Number %s -Confirm:$false -RemoveData", diskNumber))
	}
}

func (e *WindowsEnvironment) EnsureDiskCleared(diskNumber string) {
	e.EnsureDiskClearedWithOffset(1, diskNumber)
}
