// +build windows

package platform_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"

	. "github.com/cloudfoundry/bosh-agent/platform"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	"github.com/cloudfoundry/bosh-agent/platform/cert/certfakes"
	fakeplat "github.com/cloudfoundry/bosh-agent/platform/fakes"
	fakenet "github.com/cloudfoundry/bosh-agent/platform/net/fakes"
	fakestats "github.com/cloudfoundry/bosh-agent/platform/stats/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	fakeuuidgen "github.com/cloudfoundry/bosh-utils/uuid/fakes"
	"github.com/onsi/gomega/gbytes"
)

var (
	modadvapi32   = windows.NewLazySystemDLL("Advapi32.dll")
	procLogonUser = modadvapi32.NewProc("LogonUserW")
)

const ERROR_LOGON_FAILURE = syscall.Errno(0x52E)

// Use LogonUser to check if the provided password is correct.
//
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa378184(v=vs.85).aspx
//
func ValidUserPassword(username, password string) error {
	const LOGON32_LOGON_NETWORK = 3
	const LOGON32_PROVIDER_DEFAULT = 0

	if err := procLogonUser.Find(); err != nil {
		return err
	}
	puser, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	ppass, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return err
	}
	var token windows.Handle
	r1, _, e1 := syscall.Syscall6(procLogonUser.Addr(), 6,
		uintptr(unsafe.Pointer(puser)),  // LPTSTR  lpszUsername,
		uintptr(0),                      // LPTSTR  lpszDomain,
		uintptr(unsafe.Pointer(ppass)),  // LPTSTR  lpszPassword,
		LOGON32_LOGON_NETWORK,           // DWORD   dwLogonType,
		LOGON32_PROVIDER_DEFAULT,        // DWORD   dwLogonProvider,
		uintptr(unsafe.Pointer(&token)), // PHANDLE phToken
	)
	if r1 == 0 {
		if e1 == 0 {
			return syscall.EINVAL
		}
		return e1
	}
	windows.CloseHandle(token)
	return nil
}

var _ = Describe("WindowsPlatform", func() {
	var (
		collector                  *fakestats.FakeCollector
		fs                         *fakesys.FakeFileSystem
		cmdRunner                  *fakesys.FakeCmdRunner
		dirProvider                boshdirs.Provider
		netManager                 *fakenet.FakeManager
		devicePathResolver         *fakedpresolv.FakeDevicePathResolver
		options                    Options
		platform                   Platform
		fakeDefaultNetworkResolver *fakenet.FakeDefaultNetworkResolver
		fakeUUIDGenerator          *fakeuuidgen.FakeGenerator
		certManager                *certfakes.FakeManager
		auditLogger                *fakeplat.FakeAuditLogger

		formatter *fakeplat.FakeWindowsDiskFormatter
		linker    *fakeplat.FakeWindowsDiskLinker
		logger    boshlog.Logger
		logBuffer *gbytes.Buffer
	)

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		logger = boshlog.NewWriterLogger(boshlog.LevelDebug, logBuffer)

		collector = &fakestats.FakeCollector{}
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		dirProvider = boshdirs.NewProvider("/fake-dir")
		netManager = &fakenet.FakeManager{}
		devicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		options = Options{}
		fakeDefaultNetworkResolver = &fakenet.FakeDefaultNetworkResolver{}
		certManager = new(certfakes.FakeManager)
		auditLogger = fakeplat.NewFakeAuditLogger()
		fakeUUIDGenerator = fakeuuidgen.NewFakeGenerator()
		formatter = new(fakeplat.FakeWindowsDiskFormatter)
		linker = new(fakeplat.FakeWindowsDiskLinker)

		platform = NewWindowsPlatform(
			collector,
			fs,
			cmdRunner,
			dirProvider,
			netManager,
			certManager,
			devicePathResolver,
			options,
			logger,
			fakeDefaultNetworkResolver,
			auditLogger,
			fakeUUIDGenerator,
			formatter,
			linker,
		)
	})

	Describe("GetFileContentsFromCDROM", func() {
		It("reads file from D drive", func() {
			fs.WriteFileString("D:/env", "fake-contents")
			contents, err := platform.GetFileContentsFromCDROM("env")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal([]byte("fake-contents")))
		})
	})

	Describe("SetupTmpDir", func() {
		var (
			OrigTMP  = os.Getenv("TMP")
			OrigTEMP = os.Getenv("TEMP")
		)
		AfterEach(func() {
			os.Setenv("TMP", OrigTMP)
			os.Setenv("TEMP", OrigTEMP)
		})

		It("creates new temp dir", func() {
			err := platform.SetupTmpDir()
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat("/fake-dir/data/tmp")
			Expect(fileStats).NotTo(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
		})

		It("returns error if creating new temp dir errs", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-error")

			err := platform.SetupTmpDir()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
		})

		It("sets TMP and TEMP environment variable so that children of this process will use new temp dir", func() {
			err := platform.SetupTmpDir()
			Expect(err).NotTo(HaveOccurred())

			fakeTmpDir := filepath.FromSlash("/fake-dir/data/tmp")
			Expect(os.Getenv("TMP")).To(Equal(fakeTmpDir))
			Expect(os.Getenv("TEMP")).To(Equal(fakeTmpDir))
		})

		It("returns error if setting TMPDIR errs", func() {
			// uses os package; no way to trigger err
		})
	})

	Describe("SetupBlobsDir", func() {
		act := func() error {
			return platform.SetupBlobsDir()
		}

		It("creates new temp dir", func() {
			err := act()
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat("/fake-dir/data/blobs")
			Expect(fileStats).NotTo(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
		})

		It("returns error if creating new temp dir errs", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-error")

			err := act()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
		})
	})

	Describe("SetupDataDir", func() {
		It("creates new temp dir", func() {
			err := platform.SetupDataDir()
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat("/fake-dir/data/sys/log")
			Expect(fileStats).NotTo(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))

			fileStats = fs.GetFileTestStat("/fake-dir/sys")
			Expect(fileStats).NotTo(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeSymlink)))
		})

		It("returns error if creating new temp dir errs", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-error")

			err := platform.SetupDataDir()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
		})
	})

	Describe("SetupNetworking", func() {
		It("delegates to the NetManager", func() {
			networks := boshsettings.Networks{}

			err := platform.SetupNetworking(networks)
			Expect(err).ToNot(HaveOccurred())

			Expect(netManager.SetupNetworkingNetworks).To(Equal(networks))
		})
	})

	Describe("GetDefaultNetwork", func() {
		It("delegates to the defaultNetworkResolver", func() {
			defaultNetwork := boshsettings.Network{IP: "1.2.3.4"}
			fakeDefaultNetworkResolver.GetDefaultNetworkNetwork = defaultNetwork

			network, err := platform.GetDefaultNetwork()
			Expect(err).ToNot(HaveOccurred())

			Expect(network).To(Equal(defaultNetwork))
		})
	})

	Describe("SetTimeWithNtpServers", func() {
		It("sets time with ntp servers", func() {
			servers := []string{"0.north-america.pool.ntp.org", "1.north-america.pool.ntp.org"}
			platform.SetTimeWithNtpServers(servers)

			Expect(len(cmdRunner.RunCommands)).To(Equal(6))
			Expect(cmdRunner.RunCommands[0]).To(ContainElement(ContainSubstring("new-netfirewallrule")))
			Expect(cmdRunner.RunCommands[1]).To(ContainElement(ContainSubstring("stop")))
			ntpServers := strings.Join(servers, " ")
			Expect(cmdRunner.RunCommands[2]).To(ContainElement(ContainSubstring(ntpServers)))
			Expect(cmdRunner.RunCommands[2]).To(ContainElement(ContainSubstring("powershell.exe")))
			Expect(cmdRunner.RunCommands[3]).To(ContainElement(ContainSubstring("start")))
			Expect(cmdRunner.RunCommands[4]).To(ContainElement(ContainSubstring("/update")))
			Expect(cmdRunner.RunCommands[5]).To(ContainElement(ContainSubstring("/resync")))
		})

		It("sets time with ntp servers is noop when no ntp server provided", func() {
			platform.SetTimeWithNtpServers([]string{})
			Expect(len(cmdRunner.RunCommands)).To(Equal(0))
		})
	})

	Describe("DeleteARPEntryWithIP", func() {
		It("cleans the arp entry for the given ip", func() {
			err := platform.DeleteARPEntryWithIP("1.2.3.4")
			deleteArpEntry := []string{"arp", "-d", "1.2.3.4"}
			Expect(cmdRunner.RunCommands[0]).To(Equal(deleteArpEntry))
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails if arp command fails", func() {
			result := fakesys.FakeCmdResult{
				Error:      errors.New("failure"),
				ExitStatus: 1,
				Stderr:     "",
				Stdout:     "",
			}
			cmdRunner.AddCmdResult("arp -d 1.2.3.4", result)

			err := platform.DeleteARPEntryWithIP("1.2.3.4")

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("SaveDNSRecords", func() {
		var dnsRecords boshsettings.DNSRecords

		BeforeEach(func() {
			dnsRecords = boshsettings.DNSRecords{
				Records: [][2]string{
					{"fake-ip0", "fake-name0"},
					{"fake-ip1", "fake-name1"},
				},
			}
		})

		It("writes the new DNS records in '/etc/hosts'", func() {
			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).ToNot(HaveOccurred())

			windir := os.Getenv("windir")
			hostsFileContents, err := fs.ReadFile(windir + "\\System32\\Drivers\\etc\\hosts")
			Expect(err).ToNot(HaveOccurred())

			Expect(hostsFileContents).Should(MatchRegexp("fake-ip0\\s+fake-name0\\n"))
			Expect(hostsFileContents).Should(MatchRegexp("fake-ip1\\s+fake-name1\\n"))
		})

		It("renames intermediary /etc/hosts-<uuid> atomically to /etc/hosts", func() {
			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).ToNot(HaveOccurred())

			Expect(fs.RenameError).ToNot(HaveOccurred())

			// Use '/Windows' to make the fakefilesystem happy...
			Expect(len(fs.RenameOldPaths)).To(Equal(1))
			Expect(fs.RenameOldPaths).To(ContainElement("/Windows/System32/Drivers/etc/hosts-fake-uuid-0"))

			Expect(len(fs.RenameNewPaths)).To(Equal(1))
			Expect(fs.RenameNewPaths).To(ContainElement("/Windows/System32/Drivers/etc/hosts"))
		})
	})

	Describe("GetHostPublicKey", func() {
		var previous func() error

		BeforeEach(func() {
			previous = SetSSHEnabled(func() error { return nil })
		})

		AfterEach(func() {
			SetSSHEnabled(previous)
		})

		const ExpPublicKey = "PUBLIC ECDSA KEY"

		setupHostKeys := func(drive string) {
			if drive == "" {
				drive = "C:"
			}
			drive += "\\"

			dirname := filepath.Join(drive, "ProgramData", "ssh")
			fs.MkdirAll(dirname, 0744)
			var keyTypes = []string{
				"dsa",
				"ecdsa",
				"ed25519",
				"rsa",
			}
			for _, s := range keyTypes {
				name := fmt.Sprintf("ssh_host_%s_key", s)
				path := filepath.Join(dirname, name)

				fs.WriteFileString(path, fmt.Sprintf("PRIVATE %s KEY", strings.ToUpper(s)))
				path += ".pub"
				fs.WriteFileString(path, fmt.Sprintf("PUBLIC %s KEY", strings.ToUpper(s)))
			}
		}

		It("reads the host ECDSA key", func() {
			setupHostKeys(os.Getenv("SYSTEMDRIVE"))
			key, err := platform.GetHostPublicKey()
			Expect(err).ToNot(HaveOccurred())
			Expect(key).To(Equal(ExpPublicKey))
		})

		It("reads the host key stored in %SYSTEMDRIVE%\\ProgramData\\ssh", func() {
			oldSys := os.Getenv("SYSTEMDRIVE")
			defer os.Setenv("SYSTEMDRIVE", oldSys)
			newSys := "K:"
			os.Setenv("SYSTEMDRIVE", newSys)

			setupHostKeys(newSys)

			key, err := platform.GetHostPublicKey()
			Expect(err).ToNot(HaveOccurred())
			Expect(key).To(Equal(ExpPublicKey))
		})

		It("fails if the sshd daemon is not running", func() {
			setupHostKeys(os.Getenv("SYSTEMDRIVE"))

			previous := SetSSHEnabled(func() error { return errors.New("test") })
			defer SetSSHEnabled(previous)

			_, err := platform.GetHostPublicKey()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GetEphemeralDiskPath", func() {
		It("returns empty string when disk settings path is empty", func() {
			diskPath := platform.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: ""})
			Expect(diskPath).To(Equal(""))
		})

		It("returns 0 when disk settings path is empty and CreatePartitionIfNoEphemeralDisk is true", func() {
			platform = NewWindowsPlatform(
				collector,
				fs,
				cmdRunner,
				dirProvider,
				netManager,
				certManager,
				devicePathResolver,
				Options{
					Linux: LinuxOptions{
						CreatePartitionIfNoEphemeralDisk: true,
					},
				},
				logger,
				fakeDefaultNetworkResolver,
				auditLogger,
				fakeUUIDGenerator,
				formatter,
				linker,
			)

			diskPath := platform.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: ""})
			Expect(diskPath).To(Equal("0"))
		})

		It("return 1 when disk settings path is /dev/sdb", func() {
			diskPath := platform.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: "/dev/sdb"})
			Expect(diskPath).To(Equal("1"))
		})

		It("returns 2 when disk settings path is /dev/sdc", func() {
			diskPath := platform.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: "/dev/sdc"})
			Expect(diskPath).To(Equal("2"))
		})
	})

	Describe("SetupEphemeralDiskWithPath", func() {
		const (
			initializeDiskError = `Initialize-Disk : A parameter cannot be found that matches parameter name 'ParititionStyle'.
At line:1 char:27
+ Initialize-Disk -Number 1 -ParititionStyle GPT
+                           ~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidArgument: (:) [Initialize-Disk], ParameterBindingException
    + FullyQualifiedErrorId : NamedParameterNotFound,Initialize-Disk`
			partitionError = `new-partition : Not enough available capacity
At line:1 char:1
+ new-partition -DiskNumber 0 -UseMaximumSize
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : NotSpecified: (StorageWMI:ROOT/Microsoft/Windows
   /Storage/MSFT_Disk) [New-Partition], CimException
    + FullyQualifiedErrorId : StorageWMI 40000,New-Partition`
			getPartitionError = `get-partition : No MSFT_Partition objects found with property 'DiskNumber'
equal to '4'.  Verify the value of the property and retry.
At line:1 char:1
+ Get-Partition -DiskNumber 4 | Select-Object -Last 1 | Format-Volume
-FileSystem  ...
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (4:UInt32) [Get-Partition], CimJ
   obException
    + FullyQualifiedErrorId : CmdletizationQuery_NotFound_DiskNumber,Get-Parti
   tion`
			accessPathError = `add-partitionaccesspath : Invalid Parameter
At line:1 char:50
+ get-partition -disknumber 0 -partitionnumber 3 | add-partitionaccesspath
-access ...
+
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidArgument: (StorageWMI:ROOT/Microsoft/.../
   MSFT_Partition) [Add-PartitionAccessPath], CimException
    + FullyQualifiedErrorId : StorageWMI 5,Add-PartitionAccessPath
`
			getDiskError = `Get-Disk : No MSFT_Disk objects found with property 'Number' equal to '0'.
Verify the value of the property and retry.
At line:1 char:1
+ Get-Disk 0
+ ~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (0:UInt32) [Get-Disk], CimJobExc
   eption
    + FullyQualifiedErrorId : CmdletizationQuery_NotFound_Number,Get-Disk
`
			largeRemainingDiskOutput = `31404851200
`
			newLineOutput = `
`

			protectDirMissingError = `
Get-Command : The term 'Protect-MountedDir' is not recognized as the name of a cmdlet, function, script file, or operable program.
Check the spelling of the name, or if a path was included, verify that the path is correct and try again.
At line:1 char:1
+ Get-Command Protect-Dir
+ ~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : ObjectNotFound: (Protect-Dir:String) [Get-Command], CommandNotFoundException
    + FullyQualifiedErrorId : CommandNotFoundException,Microsoft.PowerShell.Commands.GetCommandCommand
`

			protectDirError = `At line:1 char:62
+ $acPath = Get-Acl -LiteralPath C:\\fake-dir\\data\\
+                                                              ~
You must provide a value expression following the '-' operator.
At line:1 char:62
+ ... et-Acl -LiteralPath C:\\fake-dir\\data\\
+                         ~~~~~~~~~~~~~~~~~
Unexpected token '80be-d2c3c2124585' in expression or statement.
    + CategoryInfo          : ParserError: (:) [], ParentContainsErrorRecordException
    + FullyQualifiedErrorId : ExpectedValueExpression
`
		)

		var (
			diskNumber, partitionNumber, partitionCount, dataDir, driveLetter string
		)

		BeforeEach(func() {
			diskNumber = "0"
			partitionNumber = "3"
			partitionCount = "0"
			driveLetter = "E"
			dataDir = fmt.Sprintf(`C:%s\`, dirProvider.DataDir())

			platform = NewWindowsPlatform(
				collector,
				fs,
				cmdRunner,
				dirProvider,
				netManager,
				certManager,
				devicePathResolver,
				Options{
					Windows: WindowsOptions{
						EnableEphemeralDiskMounting: true,
					},
				},
				logger,
				fakeDefaultNetworkResolver,
				auditLogger,
				fakeUUIDGenerator,
				formatter,
				linker,
			)
		})

		prepareSuccessfulFakeCommands := func(diskNumber, partitionNumber, dataDir, driveLetter string) {
			cmdRunner.AddCmdResult(checkProtectDirExistsCommand(), fakesys.FakeCmdResult{})
			cmdRunner.AddCmdResult(
				getDiskLargestFreeExtentCommand(diskNumber),
				fakesys.FakeCmdResult{Stdout: largeRemainingDiskOutput},
			)
			partitionNumberOutput := fmt.Sprintf(`%s
`, partitionNumber)
			cmdRunner.AddCmdResult(initializeDiskCommand(diskNumber), fakesys.FakeCmdResult{})
			partitionCountOutput := fmt.Sprintf(`%s
`, partitionCount)
			cmdRunner.AddCmdResult(
				getExistingPartitionCountCommand(diskNumber),
				fakesys.FakeCmdResult{Stdout: partitionCountOutput},
			)
			cmdRunner.AddCmdResult(newPartitionCommand(diskNumber), fakesys.FakeCmdResult{Stdout: partitionNumberOutput})
			cmdRunner.AddCmdResult(addPartitionAccessPathCommand(diskNumber, partitionNumber), fakesys.FakeCmdResult{})
			driveLetterOutput := fmt.Sprintf(`%s
`, driveLetter)
			cmdRunner.AddCmdResult(
				getDriveLetter(diskNumber, partitionNumber),
				fakesys.FakeCmdResult{Stdout: driveLetterOutput},
			)
			cmdRunner.AddCmdResult(makelinkCommand(dataDir, driveLetter), fakesys.FakeCmdResult{})
			cmdRunner.AddCmdResult(protectDirCmd(dataDir), fakesys.FakeCmdResult{})
		}

		It("does nothing when path is empty", func() {
			diskNumber = ""
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(0))
		})

		It("partitions the root disk when disk is 0", func() {
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(BeNumerically(">", 1))
			Expect(cmdRunner.RunCommands).To(ContainElement(Equal(strings.Split(newPartitionCommand(diskNumber), " "))))

			Expect(linker.IsLinkedCallCount()).To(Equal(1))
			Expect(linker.IsLinkedArgsForCall(0)).To(Equal(dataDir))
			expectFormatterCalledWithArgs(formatter, diskNumber, partitionNumber)

			Expect(cmdRunner.RunCommands).To(ContainElement(Equal(
				strings.Split(addPartitionAccessPathCommand(diskNumber, partitionNumber), " "),
			)))
			Expect(cmdRunner.RunCommands).To(ContainElement(Equal(strings.Split(protectDirCmd(dataDir), " "))))
			Expect(cmdRunner.RunCommands).NotTo(ContainElement(Equal(strings.Split(initializeDiskCommand(diskNumber), " "))))
			Expect(cmdRunner.RunCommands).NotTo(ContainElement(Equal(strings.Split(getExistingPartitionCountCommand(diskNumber), " "))))
		})

		It("partitions an attached disk when disk is 1", func() {
			diskNumber = "1"
			partitionNumber = "1"
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(BeNumerically(">", 1))
			Expect(cmdRunner.RunCommands).To(Equal([][]string{
				strings.Split(checkProtectDirExistsCommand(), " "),
				strings.Split(getExistingPartitionCountCommand(diskNumber), " "),
				strings.Split(initializeDiskCommand(diskNumber), " "),
				strings.Split(getDiskLargestFreeExtentCommand(diskNumber), " "),
				strings.Split(newPartitionCommand(diskNumber), " "),
				strings.Split(addPartitionAccessPathCommand(diskNumber, partitionNumber), " "),
				strings.Split(getDriveLetter(diskNumber, partitionNumber), " "),
				strings.Split(makelinkCommand(dataDir, driveLetter), " "),
				strings.Split(protectDirCmd(dataDir), " "),
			}))

			expectFormatterCalledWithArgs(formatter, diskNumber, partitionNumber)
		})

		It("does nothing if partition exists and is linked to data dir", func() {
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.IsLinkedReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdRunner.RunCommands).NotTo(ContainElement(Equal(strings.Split(newPartitionCommand(diskNumber), " "))))
		})

		It("doesn't warn about low disk space if partition exists and is linked to data dir", func() {
			zeroRemainingDiskOutput := `0
`

			cmdRunner.AddCmdResult(
				getDiskLargestFreeExtentCommand(diskNumber),
				fakesys.FakeCmdResult{Stdout: zeroRemainingDiskOutput},
			)
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.IsLinkedReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Consistently(logBuffer).ShouldNot(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(cmdRunner.RunCommands).NotTo(ContainElement(Equal(strings.Split(newPartitionCommand(diskNumber), " "))))
		})

		It("logs a warning and doesn't create a partition if there is less than 1MB of free disk space", func() {
			smallRemainingDiskOutput := fmt.Sprintf(`%s
`, (1024*1024)-1)
			cmdRunner.AddCmdResult(
				getDiskLargestFreeExtentCommand(diskNumber),
				fakesys.FakeCmdResult{Stdout: smallRemainingDiskOutput},
			)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(cmdRunner.RunCommands).NotTo(ContainElement(Equal(strings.Split(newPartitionCommand(diskNumber), " "))))
		})

		It("returns an error a warning when Protect-Dir cmdlet is missing", func() {
			cmdRunner.AddCmdResult(
				checkProtectDirExistsCommand(),
				fakesys.FakeCmdResult{ExitStatus: 1, Stderr: protectDirMissingError},
			)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)
			Expect(err).To(MatchError(
				fmt.Sprintf("Cannot protect %s. Protect-Path cmd does not exist: %s", dataDir, protectDirMissingError),
			))

		})

		It("returns an error when getting free disk space command fails", func() {
			cmdRunnerError := errors.New("It went wrong")
			expandedCommand := getDiskLargestFreeExtentCommand(diskNumber)

			cmdRunner.AddCmdResult(expandedCommand, fakesys.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(
				fmt.Sprintf("Failed to run command \"%s\": %s", expandedCommand, cmdRunnerError.Error()),
			))
		})

		It("returns an error when Get-Disk command returns non-zero exit code", func() {
			cmdStderr := getDiskError
			cmdRunner.AddCmdResult(
				getDiskLargestFreeExtentCommand(diskNumber),
				fakesys.FakeCmdResult{Stderr: cmdStderr, ExitStatus: 197},
			)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(
				fmt.Sprintf("Failed to get free disk space on disk %s: %s", diskNumber, cmdStderr),
			))
		})

		It("returns an error when Getting existing partition check command fails", func() {
			isLinkedError := errors.New("It went wrong")
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.IsLinkedReturns("", isLinkedError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(isLinkedError))
		})

		It("returns an error when Get-Disk NumberOfPartitions command return non-zero exit code", func() {
			diskNumber = "1"
			partitionNumber = "1"

			cmdStderr := getDiskError
			cmdRunner.AddCmdResult(getExistingPartitionCountCommand(diskNumber), fakesys.FakeCmdResult{Stderr: cmdStderr, ExitStatus: 197})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to get existing partition count for disk %s: %s", diskNumber, cmdStderr)))
		})

		It("returns an error when Get-Disk NumberOfPartitions command fails", func() {
			diskNumber = "1"
			partitionNumber = "1"

			cmdRunnerError := errors.New("It went wrong")
			expandedCommand := getExistingPartitionCountCommand(diskNumber)
			cmdRunner.AddCmdResult(expandedCommand, fakesys.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to run command \"%s\": %s", expandedCommand, cmdRunnerError.Error())))
		})

		It("returns an error when Initialize-Disk command return non-zero exit code", func() {
			diskNumber = "1"
			partitionNumber = "1"

			cmdStderr := initializeDiskError
			cmdRunner.AddCmdResult(initializeDiskCommand(diskNumber), fakesys.FakeCmdResult{Stderr: cmdStderr, ExitStatus: 197})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to initialize disk %s: %s", diskNumber, cmdStderr)))
		})

		It("returns an error when Initialize-Disk command fails", func() {
			diskNumber = "1"
			partitionNumber = "1"

			cmdRunnerError := errors.New("It went wrong")
			expandedCommand := initializeDiskCommand(diskNumber)
			cmdRunner.AddCmdResult(expandedCommand, fakesys.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to run command \"%s\": %s", expandedCommand, cmdRunnerError.Error())))
		})

		It("returns an error when New-Partition command returns non-zero exit code", func() {
			cmdStderr := partitionError
			cmdRunner.AddCmdResult(newPartitionCommand(diskNumber), fakesys.FakeCmdResult{Stderr: cmdStderr, ExitStatus: 197})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to create partition on disk 0: %s", cmdStderr)))
		})

		It("returns an error when running new-partition command fails", func() {
			cmdRunnerError := errors.New("It went wrong")
			expandedCommand := newPartitionCommand(diskNumber)
			cmdRunner.AddCmdResult(expandedCommand, fakesys.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(
				fmt.Sprintf("Failed to run command \"%s\": %s", expandedCommand, cmdRunnerError.Error()),
			))
		})

		It("returns an error when format command fails", func() {
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)

			formatError := errors.New("A failure occurred")
			formatter.FormatReturns(formatError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(formatError))
		})

		It("Returns an error when Add-PartitionAccessPath fails", func() {
			cmdStderr := accessPathError
			cmdRunner.AddCmdResult(
				addPartitionAccessPathCommand(diskNumber, partitionNumber),
				fakesys.FakeCmdResult{Stderr: cmdStderr, ExitStatus: 197},
			)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(
				fmt.Sprintf(
					"Failed to assign drive letter to partition %s for device %s: %s",
					partitionNumber, diskNumber, cmdStderr,
				),
			))
		})

		It("returns an error when attempting to add partition access path command fails", func() {
			cmdRunnerError := errors.New("Failure")
			expandedCommand := addPartitionAccessPathCommand(diskNumber, partitionNumber)
			cmdRunner.AddCmdResult(expandedCommand, fakesys.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to run command \"%s\": %s", expandedCommand, cmdRunnerError)))
		})

		It("Returns an error when Protect-MountedDir fails", func() {
			cmdStderr := protectDirError
			cmdRunner.AddCmdResult(protectDirCmd(dataDir), fakesys.FakeCmdResult{ExitStatus: 197, Stderr: cmdStderr})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(
				fmt.Sprintf("Failed to protect path %s : %s", dataDir, cmdStderr),
			))
		})

		It("returns an error when calling protect-dir command fails", func() {
			cmdRunnerError := errors.New("Failure")
			expandedCommand := protectDirCmd(dataDir)
			cmdRunner.AddCmdResult(expandedCommand, fakesys.FakeCmdResult{ExitStatus: -1, Error: cmdRunnerError})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(fmt.Sprintf("Failed to run command \"%s\": %s", expandedCommand, cmdRunnerError)))
		})

		It("when the ephemeral disk mounting feature flag is not present doesn't do anything", func() {
			platform = NewWindowsPlatform(
				collector,
				fs,
				cmdRunner,
				dirProvider,
				netManager,
				certManager,
				devicePathResolver,
				Options{},
				logger,
				fakeDefaultNetworkResolver,
				auditLogger,
				fakeUUIDGenerator,
				formatter,
				linker,
			)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Consistently(logBuffer).ShouldNot(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(cmdRunner.RunCommands).To(BeEmpty())
		})
	})
})

func getExistingPartitionCountCommand(diskNumber string) string {
	return fmt.Sprintf("powershell.exe Get-Disk -Number %s | Select -ExpandProperty NumberOfPartitions", diskNumber)
}

func initializeDiskCommand(diskNumber string) string {
	return fmt.Sprintf("powershell.exe Initialize-Disk -Number %s -PartitionStyle GPT", diskNumber)
}

func newPartitionCommand(diskNumber string) string {
	return fmt.Sprintf(
		"powershell.exe New-Partition -DiskNumber %s -UseMaximumSize | Select -ExpandProperty PartitionNumber",
		diskNumber,
	)
}

func addPartitionAccessPathCommand(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		"powershell.exe Add-PartitionAccessPath -DiskNumber %s -PartitionNumber %s -AssignDriveLetter",
		diskNumber, partitionNumber,
	)
}

func getDriveLetter(diskNumber, partitionNumber string) string {
	return fmt.Sprintf(
		`powershell.exe Get-Partition -DiskNumber %s -PartitionNumber %s | Select -ExpandProperty DriveLetter`,
		diskNumber,
		partitionNumber,
	)
}

func makelinkCommand(dataPath, driveLetter string) string {
	return fmt.Sprintf(`powershell.exe cmd.exe /c mklink /D %s %s:`, dataPath, driveLetter)
}

func protectDirCmd(dataDir string) string {
	removedTrailingSlash := strings.TrimRight(dataDir, "\\")
	return fmt.Sprintf(
		`powershell.exe Protect-Path '%s'`,
		removedTrailingSlash,
	)
}

func getDiskLargestFreeExtentCommand(diskNumber string) string {
	return fmt.Sprintf(`powershell.exe Get-Disk %s | Select -ExpandProperty LargestFreeExtent`, diskNumber)
}

func checkProtectDirExistsCommand() string {
	return "powershell.exe Get-Command Protect-Path"
}

func expectFormatterCalledWithArgs(
	formatter *fakeplat.FakeWindowsDiskFormatter,
	expectedDiskNumber,
	expectedPartitionNumber string,
) {

	ExpectWithOffset(1, formatter.FormatCallCount()).To(Equal(1))
	diskNumber, partitionNumber := formatter.FormatArgsForCall(0)
	ExpectWithOffset(1, diskNumber).To(Equal(expectedDiskNumber))
	ExpectWithOffset(1, partitionNumber).To(Equal(expectedPartitionNumber))
}

var _ = Describe("BOSH User Commands", func() {
	const testUsername = boshsettings.EphemeralUserPrefix + "test_abc123"

	var (
		// We're doing this for real - no fakes!
		logger    = boshlog.NewLogger(boshlog.LevelNone)
		fs        = boshsys.NewOsFileSystem(logger)
		cmdRunner = boshsys.NewExecCmdRunner(logger)

		deleteUserOnce sync.Once
	)

	BeforeEach(func() {
		deleteUserOnce.Do(func() {
			DeleteUserProfile(testUsername)
		})
	})

	userExists := func(name string) error {
		_, _, t, err := syscall.LookupSID("", name)
		if err != nil {
			return err
		}
		if t != syscall.SidTypeUser {
			return fmt.Errorf("not a user sid: %s", name)
		}
		return nil
	}

	AfterEach(func() {
		DeleteUserProfile(testUsername)
		Expect(userExists(testUsername)).ToNot(Succeed())
	})

	Describe("SSH", func() {

		var platform Platform

		BeforeEach(func() {
			var (
				collector                  = &fakestats.FakeCollector{}
				netManager                 = &fakenet.FakeManager{}
				devicePathResolver         = fakedpresolv.NewFakeDevicePathResolver()
				fakeDefaultNetworkResolver = &fakenet.FakeDefaultNetworkResolver{}
				certManager                = new(certfakes.FakeManager)
				auditLogger                = fakeplat.NewFakeAuditLogger()
				fakeUUIDGenerator          = fakeuuidgen.NewFakeGenerator()
				dirProvider                = boshdirs.NewProvider("/fake-dir")
				formatter                  = new(fakeplat.FakeWindowsDiskFormatter)
				linker                     = new(fakeplat.FakeWindowsDiskLinker)
			)
			platform = NewWindowsPlatform(
				collector,
				fs,
				cmdRunner,
				dirProvider,
				netManager,
				certManager,
				devicePathResolver,
				Options{},
				logger,
				fakeDefaultNetworkResolver,
				auditLogger,
				fakeUUIDGenerator,
				formatter,
				linker,
			)
		})

		It("can create a user with Admin privileges", func() {
			Expect(platform.CreateUser(testUsername, "")).To(Succeed())
			Expect(userExists(testUsername)).To(Succeed())

			cmd := exec.Command("NET.exe", "LOCALGROUP", "Administrators")
			out, err := cmd.CombinedOutput()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(out)).To(ContainSubstring(testUsername))
		})

		sshdServiceIsInstalled := func() bool {
			m, err := mgr.Connect()
			if err != nil {
				return false
			}
			defer m.Disconnect()
			s, err := m.OpenService("sshd")
			if err != nil {
				return false
			}
			s.Close()
			return true
		}

		It("can insert public keys into the users .ssh\\authorized_keys file", func() {
			if !sshdServiceIsInstalled() {
				Skip("This test requires the SSHD service to be installed")
			}

			keys := []string{
				"KEY_1",
				"KEY_2",
				"KEY_3",
			}
			Expect(platform.CreateUser(testUsername, "")).To(Succeed())
			Expect(userExists(testUsername)).To(Succeed())

			Expect(platform.SetupSSH(keys, testUsername)).To(Succeed())

			homedir, err := UserHomeDirectory(testUsername)
			Expect(err).To(Succeed())

			keyPath := filepath.Join(homedir, ".ssh", "authorized_keys")
			b, err := ioutil.ReadFile(keyPath)
			Expect(err).To(Succeed())

			content := strings.TrimSpace(string(b))
			for i, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				Expect(line).To(Equal(keys[i]))
			}
		})

		It("can delete a users matching a regex", func() {
			Expect(platform.CreateUser(testUsername, "")).To(Succeed())
			Expect(userExists(testUsername)).To(Succeed())

			homedir, err := UserHomeDirectory(testUsername)
			Expect(err).To(Succeed())

			// Regex taken from: github.com/cloudfoundry/bosh-cli/director/ssh.go
			//
			const regex = "^" + testUsername
			Expect(platform.DeleteEphemeralUsersMatching(regex)).To(Succeed())
			Expect(userExists(testUsername)).ToNot(Succeed())

			_, err = os.Stat(homedir)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Set Random Password", func() {
		const testPassword = "Password123!"

		var (
			platform      Platform
			tempDir       string
			lockFile      string
			previousAdmin string
		)

		BeforeEach(func() {
			previousAdmin = SetAdministratorUserName(testUsername)

			var err error
			tempDir, err = ioutil.TempDir("", "bosh-tests-")
			Expect(err).ToNot(HaveOccurred())

			var (
				collector                  = &fakestats.FakeCollector{}
				netManager                 = &fakenet.FakeManager{}
				devicePathResolver         = fakedpresolv.NewFakeDevicePathResolver()
				fakeDefaultNetworkResolver = &fakenet.FakeDefaultNetworkResolver{}
				certManager                = new(certfakes.FakeManager)
				auditLogger                = fakeplat.NewFakeAuditLogger()
				fakeUUIDGenerator          = fakeuuidgen.NewFakeGenerator()
				dirProvider                = boshdirs.NewProvider(tempDir)
				formatter                  = new(fakeplat.FakeWindowsDiskFormatter)
				linker                     = new(fakeplat.FakeWindowsDiskLinker)
			)
			platform = NewWindowsPlatform(
				collector,
				fs,
				cmdRunner,
				dirProvider,
				netManager,
				certManager,
				devicePathResolver,
				Options{},
				logger,
				fakeDefaultNetworkResolver,
				auditLogger,
				fakeUUIDGenerator,
				formatter,
				linker,
			)

			lockFile = filepath.Join(platform.GetDirProvider().BoshDir(), "randomized_passwords")
		})

		AfterEach(func() {
			SetAdministratorUserName(previousAdmin)
			os.RemoveAll(tempDir)
		})

		Context("Called on vcap or root user", func() {
			It("it does nothing if the Administrator user does not exist", func() {
				Expect(lockFile).ToNot(BeAnExistingFile())

				Expect(platform.SetUserPassword(boshsettings.VCAPUsername, testPassword)).To(Succeed())
				Expect(lockFile).To(BeAnExistingFile())
				fs.RemoveAll(lockFile)

				Expect(platform.SetUserPassword(boshsettings.RootUsername, testPassword)).To(Succeed())
				Expect(lockFile).To(BeAnExistingFile())
				fs.RemoveAll(lockFile)
			})

			It("sets a random password on the Administrator user if it exists", func() {
				// create the testuser
				Expect(platform.CreateUser(testUsername, "")).To(Succeed())
				Expect(userExists(testUsername)).To(Succeed())

				rootUsers := []string{
					boshsettings.VCAPUsername,
					boshsettings.RootUsername,
				}
				for _, root := range rootUsers {
					Expect(lockFile).ToNot(BeAnExistingFile())

					cmd := exec.Command("NET.exe", "USER", testUsername, testPassword)
					Expect(cmd.Run()).To(Succeed())

					Expect(platform.SetUserPassword(root, "")).To(Succeed())

					err := ValidUserPassword(testUsername, testPassword)
					Expect(err).ToNot(Succeed(),
						fmt.Sprintf("Testing with Root user: %s", root))
					Expect(err).To(Equal(ERROR_LOGON_FAILURE),
						fmt.Sprintf("Testing with Root user: %s", root))

					Expect(lockFile).To(BeAnExistingFile())
					fs.RemoveAll(lockFile)
				}
			})

			It("sets the Admin password only once", func() {
				Expect(platform.CreateUser(testUsername, "")).To(Succeed())
				Expect(userExists(testUsername)).To(Succeed())

				Expect(platform.SetUserPassword(boshsettings.VCAPUsername, "")).To(Succeed())
				Expect(lockFile).To(BeAnExistingFile())

				// Set password to a known value
				out, err := exec.Command("NET.exe", "USER", testUsername, testPassword).CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), "NET.exe output: "+string(out))

				Expect(platform.SetUserPassword(boshsettings.VCAPUsername, "")).To(Succeed())

				Expect(ValidUserPassword(testUsername, testPassword)).To(Succeed(),
					"The second call to SetUserPassword should NOT have changed the "+
						"password for user: "+testUsername)
			})
		})

		Context("Called on user that is not vcap or root", func() {
			It("Does NOT change the Administrator user password", func() {
				Expect(platform.CreateUser(testUsername, "")).To(Succeed())
				Expect(userExists(testUsername)).To(Succeed())

				out, err := exec.Command("NET.exe", "USER", testUsername, testPassword).CombinedOutput()
				Expect(err).ToNot(HaveOccurred(), "NET.exe output: "+string(out))

				Expect(platform.SetUserPassword(testUsername, "")).To(Succeed())

				Expect(ValidUserPassword(testUsername, testPassword)).To(Succeed())

				Expect(lockFile).ToNot(BeAnExistingFile())
			})
		})
	})
})

var _ = Describe("Windows Syscalls and Helper functions", func() {
	It("Generates valid Windows passwords", func() {
		// 100,000 iterations takes about 140ms to run in a VM.
		for i := 0; i < 100000; i++ {
			s, err := RandomPassword()
			Expect(err).To(BeNil())
			Expect(s).To(HaveLen(14))
			Expect(s).ToNot(ContainSubstring("/"))
			Expect(ValidWindowsPassword(s)).To(BeTrue())
		}
	})

	expectedUserNames := func() ([]string, error) {
		cmd := exec.Command("PowerShell", "-Command",
			"Get-WmiObject -Class Win32_UserAccount | foreach { $_.Name }")

		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}
		exp := strings.Fields(string(out))
		sort.Strings(exp)
		return exp, nil
	}

	It("Lists local user accounts", func() {
		exp, err := expectedUserNames()
		Expect(err).To(Succeed())

		names, err := LocalAccountNames()
		Expect(err).To(Succeed())

		sort.Strings(names)
		Expect(names).To(Equal(exp))
	})

	It("Does not fail in a tight loop", func() {
		var wg sync.WaitGroup
		numCPU := runtime.NumCPU()
		if numCPU > 4 {
			numCPU = 4
		}
		for i := 0; i < numCPU; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 5000; i++ {
					names, err := LocalAccountNames()
					Expect(err).To(Succeed())
					Expect(names).ToNot(HaveLen(0))
				}
			}()
		}
		wg.Wait()
	})
})
