package windows_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
)

const dataDir = `C:\var\vcap\data\`

const boshAgentLogfile = `C:\var\vcap\bosh\log\service_wrapper.err.log`

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
		stdout, stderr, exitCode, err := e.RunPowershellCommandWithOffsetAndResponses(cmd)

		if err != nil {
			if strings.Contains(err.Error(), retryableError) {
				fmt.Printf("WinRM timed out on attempt %d of 5, waiting 5 seconds to retry command: %s\n", i+1, cmd)
				time.Sleep(5 * time.Second)
				continue
			} else {
				Expect(err).WithOffset(1).NotTo(
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
				Expect(exitCode).WithOffset(1).To(
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
	Expect(err).WithOffset(1).NotTo(HaveOccurred())

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

	Expect(err).WithOffset(offset + 1).NotTo(HaveOccurred())
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
	Expect(e.IsLinkTargetedToDisk(path, diskNumber)).WithOffset(1).To(BeTrue())
}

func (e *WindowsEnvironment) WaitForLink(path string) {
	e.WaitForLinkWithOffset(1, path)
}

func (e *WindowsEnvironment) WaitForLinkWithOffset(offset int, path string) {
	Eventually(func() bool {
		target, _ := e.Linker.LinkTarget(path) //nolint:errcheck
		return target != ""
	}).
		WithOffset(offset + 1).
		WithTimeout(2 * time.Minute).
		WithPolling(5 * time.Second).
		Should(BeTrue())
}

func (e *WindowsEnvironment) CleanUpUpdateSettings() {
	agent.RunPowershellCommand(`rm c:\var\vcap\bosh\update_settings.json`)
}

func (e *WindowsEnvironment) StartAgent() {
	e.RunPowershellCommandWithOffset(1, fmt.Sprintf(`If (Test-Path %s) { Remove-Item -Force -Path %s }`, boshAgentLogfile, boshAgentLogfile))
	agent.RunPowershellCommand(`c:\bosh\service_wrapper.exe install`)
	agent.RunPowershellCommand(`c:\bosh\service_wrapper.exe start`)
	Eventually(func() bool {
		return e.CheckAgentRunning(1)
	}).
		WithOffset(1).
		WithTimeout(1*time.Minute).
		WithPolling(5*time.Second).
		Should(
			BeTrue(),
			fmt.Sprintf("Expected agent to be running but it was not found:\n%s", e.RunPowershellCommandWithOffset(1, "Get-Service")),
		)

	Eventually(func() bool {
		heartBeatFound :=
			e.RunPowershellCommandWithOffset(1, fmt.Sprintf(`Get-Content -Path %s | Select-String -Quiet -Pattern "Attempting to send Heartbeat"`, boshAgentLogfile))

		return strings.TrimSpace(heartBeatFound) == "True"
	}).
		WithOffset(1).
		WithTimeout(3*time.Minute).
		WithPolling(10*time.Second).
		Should(
			BeTrue(),
			fmt.Sprintf("Expectd to see heardbeat in the logss but found:\n%s", e.RunPowershellCommandWithOffset(1, fmt.Sprintf(`Get-Content -Path %s`, boshAgentLogfile))),
		)
}

func (e *WindowsEnvironment) CheckAgentRunning(offset int) bool {
	stdout := e.RunPowershellCommandWithOffset(offset+1, "Get-Service -Name bosh-agent | Format-List -Property Status")
	running, err := regexp.MatchString("Running", strings.TrimSpace(stdout))
	Expect(err).WithOffset(offset + 1).NotTo(HaveOccurred())
	return running
}

func (e *WindowsEnvironment) EnsureAgentServiceStopped() {
	if e.CheckAgentRunning(1) {
		e.RunPowershellCommandWithOffset(1, `c:\bosh\service_wrapper.exe stop`)
	}
	e.RunPowershellCommandWithOffset(1, `c:\bosh\service_wrapper.exe uninstall`)
	e.RunPowershellCommandWithOffset(1, fmt.Sprintf(`Remove-Item -Force -Path %s`, boshAgentLogfile))
}

func (e *WindowsEnvironment) EnsureDataDirDoesntExist() {
	testPathOutput := e.RunPowershellCommandWithOffset(1, fmt.Sprintf("Test-Path -Path %s", dataDir))

	exists := strings.TrimSpace(testPathOutput) == "True"
	if exists {
		e.RunPowershellCommandWithOffset(1, fmt.Sprintf("cmd.exe /c rmdir /s /q %s", dataDir))
	}
}

func (e *WindowsEnvironment) AgentProcessRunning() bool {
	exitCode, err :=
		e.Client.RunWithContext(
			context.Background(),
			winrm.Powershell("Get-Process -ProcessName bosh-agent"),
			io.Discard, io.Discard,
		)
	return exitCode == 0 && err == nil
}

func (e *WindowsEnvironment) RunPowershellCommandWithOffset(offset int, cmd string) string {
	outString, errString, exitCode, err := e.RunPowershellCommandWithOffsetAndResponses(cmd)

	Expect(err).WithOffset(offset+1).NotTo(
		HaveOccurred(),
		fmt.Sprintf(`Command "%s" failed with stdout: %s; stderr: %s`, cmd, outString, errString),
	)
	Expect(exitCode).WithOffset(offset+1).To(
		BeZero(),
		fmt.Sprintf(
			`Command "%s" failed with exit code: %d; stdout: %s; stderr: %s`, cmd, exitCode, outString, errString,
		),
	)

	return outString
}

func (e *WindowsEnvironment) RunPowershellCommandWithOffsetAndResponses(cmd string) (string, string, int, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := e.Client.RunWithContext(context.Background(), winrm.Powershell(cmd), stdout, stderr)

	return stdout.String(), stderr.String(), exitCode, err
}

func (e *WindowsEnvironment) RunPowershellCommandWithResponses(cmd string) (string, string, int, error) {
	return e.RunPowershellCommandWithOffsetAndResponses(cmd)
}

func (e *WindowsEnvironment) RunPowershellCommand(cmd string) string {
	return e.RunPowershellCommandWithOffset(1, cmd)
}

func (e *WindowsEnvironment) AssertDataACLed() {
	// NOTE: no file separator is used below because `dataDir` has a trailing `\`
	// NOTE: filepath.Join() _CAN_NOT_ be used because the ginkgo code executes on linux, resulting in `/`
	testFile := fmt.Sprintf("%s%s", dataDir, fmt.Sprintf("test-file-%s", time.Now().Format("2006-01-02T15h04s05")))
	e.RunPowershellCommandWithOffset(1, fmt.Sprintf("New-Item -Value 'AssertDataACLed test content' -Path %s", testFile))

	checkACLsOutput := e.RunPowershellCommandWithOffset(1, fmt.Sprintf("Check-Acls %s", dataDir))
	aclErrsCount := strings.Count(checkACLsOutput, "Error")
	Expect(aclErrsCount).WithOffset(1).To(
		BeZero(),
		fmt.Sprintf(
			"Expected data directory to have correct ACLs. Counted %d errors.\n'Check-Acls %s' command output:\n%s\n-----\n",
			aclErrsCount,
			dataDir,
			checkACLsOutput,
		),
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
	Expect(err).WithOffset(offset + 1).NotTo(HaveOccurred())

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
