package windows_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/masterzen/winrm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EphemeralDisk", func() {
	var (
		client          *winrm.Client
		partitionNumber string
	)

	BeforeEach(func() {
		endpoint := winrm.NewEndpoint(os.Getenv("AGENT_ELASTIC_IP"), 5985, false, false, nil, nil, nil, 0)
		var err error
		client, err = winrm.NewClient(endpoint, "vagrant", "Password123!")
		Expect(err).NotTo(HaveOccurred())
		partitionNumber = ""
	})

	AfterEach(func() {
		if client != nil {
			ensureBoshAgentStopped(client)
			ensureFolderDoesntExist(client, "C:\\var\\vcap\\data")

			if partitionNumber != "" {
				runPowershellCommand(
					client,
					fmt.Sprintf("Remove-Partition -DiskNumber 0 -PartitionNumber %s -Confirm:$false", partitionNumber),
				)
			}
			runPowershellCommand(
				client,
				"Resize-Partition -DriveLetter C -Size $(Get-PartitionSupportedSize -DriveLetter C).SizeMax",
			)

			runPowershellCommand(client, "cp c:\\bosh\\agent-configuration\\agent.json c:\\bosh\\agent.json")
			runPowershellCommand(client, "c:\\bosh\\service_wrapper.exe start")
		}
	})

	It("when root disk can be used as ephemeral, creates a partition on root disk", func() {
		ensureBoshAgentStopped(client)
		ensureFolderDoesntExist(client, "C:\\var\\vcap\\data")

		shrinkRootPartition(client)
		runPowershellCommand(client, "cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		runPowershellCommand(client, "c:\\bosh\\service_wrapper.exe start")

		ensureVolumeHasAccessPathOnDisk(client, `C:\var\vcap\data\`, "0")

		partitionNumber = getPartitionNumberForAccessPath(client, `C:\var\vcap\data\`)

	})

	It("when root disk partition is already mounted, agent restart doesn't fail and doesn't create a new partition", func() {
		ensureBoshAgentStopped(client)
		ensureFolderDoesntExist(client, "C:\\var\\vcap\\data")

		shrinkRootPartition(client)
		runPowershellCommand(client, "cp c:\\bosh\\agent-configuration\\root-partition-agent.json c:\\bosh\\agent.json")

		runPowershellCommand(client, "c:\\bosh\\service_wrapper.exe start")

		ensureVolumeHasAccessPathOnDisk(client, `C:\var\vcap\data\`, "0")

		partitionNumber = getPartitionNumberForAccessPath(client, `C:\var\vcap\data\`)

		runPowershellCommand(client, "c:\\bosh\\service_wrapper.exe restart")

		Consistently(
			func() bool {
				exitCode, err := client.Run(
					winrm.Powershell("Get-Process -ProcessName bosh-agent"),
					ioutil.Discard, ioutil.Discard,
				)
				return exitCode == 0 && err == nil
			},
			60*time.Second,
		).Should(BeTrue(), fmt.Sprint(`Expected bosh-agent to be running after restart`))
	})
})

func shrinkRootPartition(client *winrm.Client) {
	runPowershellCommand(
		client,
		"Get-Partition -DriveLetter C | Resize-Partition -Size $(Get-PartitionSupportedSize -DriveLetter C).SizeMin",
	)
}

func getPartitionNumberForAccessPath(client *winrm.Client, accessPath string) string {
	return strings.TrimSpace(runPowershellCommand(
		client,
		fmt.Sprintf(`Get-Partition | Where AccessPaths -Contains "%s" | Select -ExpandProperty PartitionNumber`, accessPath),
	))
}

func ensureVolumeHasAccessPathOnDisk(client *winrm.Client, accessPath, diskNumber string) {
	EventuallyWithOffset(
		1,
		func() bool {
			stdout := runPowershellCommand(
				client,
				fmt.Sprintf(
					`Get-Partition | where AccessPaths -Contains "%s" | format-list -property DiskNumber`,
					accessPath,
				),
			)

			matched, err := regexp.MatchString(fmt.Sprintf(`DiskNumber : %s`, diskNumber), stdout)
			Expect(err).NotTo(HaveOccurred())
			return matched
		},
		60*time.Second,
	).Should(
		BeTrue(),
		fmt.Sprintf(
			`Expected partition with access path %s to be present on disk %s`,
			accessPath, diskNumber,
		),
	)
}

func ensureBoshAgentStopped(client *winrm.Client) {
	stdout := runPowershellCommand(client, "Get-Service -Name bosh-agent | Format-List -Property Status")

	running, err := regexp.MatchString("Running", stdout)
	Expect(err).NotTo(HaveOccurred())
	if running {
		runPowershellCommand(client, "c:\\bosh\\service_wrapper.exe stop")
	}
}

func ensureFolderDoesntExist(client *winrm.Client, path string) {
	testPathOutput := runPowershellCommand(client, "Test-Path -Path %s", path)

	exists := strings.TrimSpace(testPathOutput) == "True"
	if exists {
		runPowershellCommand(client, "Remove-Item %s -Force -Recurse", path)
	}
}

func runPowershellCommand(client *winrm.Client, cmd string, cmdFmtArgs ...interface{}) string {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode, err := client.Run(winrm.Powershell(fmt.Sprintf(cmd, cmdFmtArgs...)), stdout, stderr)

	outString := stdout.String()
	errString := stderr.String()

	Expect(err).NotTo(
		HaveOccurred(),
		fmt.Sprintf(`Command "%s" failed with stdout: %s; stderr: %s`, cmd, outString, errString),
	)
	Expect(exitCode).To(
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
