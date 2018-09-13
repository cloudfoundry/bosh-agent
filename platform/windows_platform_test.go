// +build windows

package platform_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	fakedisk "github.com/cloudfoundry/bosh-agent/platform/windows/disk/fakes"
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

		diskManager *fakeplat.FakeWindowsDiskManager
		formatter   *fakedisk.FakeWindowsDiskFormatter
		linker      *fakedisk.FakeWindowsDiskLinker
		partitioner *fakedisk.FakeWindowsDiskPartitioner
		protector   *fakedisk.FakeWindowsDiskProtector
		logger      boshlog.Logger
		logBuffer   *gbytes.Buffer
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
		diskManager = new(fakeplat.FakeWindowsDiskManager)
		formatter = new(fakedisk.FakeWindowsDiskFormatter)
		linker = new(fakedisk.FakeWindowsDiskLinker)
		partitioner = new(fakedisk.FakeWindowsDiskPartitioner)
		protector = new(fakedisk.FakeWindowsDiskProtector)

		partitioner.GetCountOnDiskReturns("0", nil)
		protector.CommandExistsReturns(true)
		diskManager.GetFormatterReturns(formatter)
		diskManager.GetLinkerReturns(linker)
		diskManager.GetPartitionerReturns(partitioner)
		diskManager.GetProtectorReturns(protector)

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
			diskManager,
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
				diskManager,
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
			protectPathError = `At line:1 char:62
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
			diskNumber, partitionNumber, dataDir, driveLetter string
		)

		BeforeEach(func() {
			diskNumber = "0"
			partitionNumber = "3"
			driveLetter = "E"
			dataDir = fmt.Sprintf(`C:%s\`, dirProvider.DataDir())

			partitioner.GetFreeSpaceOnDiskReturns(31404851200, nil)
			partitioner.PartitionDiskReturns(partitionNumber, nil)

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
				diskManager,
			)
		})

		prepareSuccessfulFakeCommands := func(diskNumber, partitionNumber, dataDir, driveLetter string) {
			cmdRunner.AddCmdResult(addPartitionAccessPathCommand(diskNumber, partitionNumber), fakesys.FakeCmdResult{})
			driveLetterOutput := fmt.Sprintf(`%s
`, driveLetter)
			cmdRunner.AddCmdResult(
				getDriveLetter(diskNumber, partitionNumber),
				fakesys.FakeCmdResult{Stdout: driveLetterOutput},
			)
			cmdRunner.AddCmdResult(protectPathCmd(dataDir), fakesys.FakeCmdResult{})
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

			Expect(protector.CommandExistsCallCount()).To(Equal(1))
			Expect(partitioner.InitializeDiskCallCount()).To(Equal(0))

			Expect(len(cmdRunner.RunCommands)).To(BeNumerically(">", 1))

			Expect(linker.LinkTargetCallCount()).To(Equal(1))
			Expect(linker.LinkTargetArgsForCall(0)).To(Equal(dataDir))

			Expect(partitioner.GetFreeSpaceOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetFreeSpaceOnDiskArgsForCall(0)).To(Equal(diskNumber))

			Expect(partitioner.PartitionDiskCallCount()).To(Equal(1))
			Expect(partitioner.PartitionDiskArgsForCall(0)).To(Equal(diskNumber))

			expectLinkCalledWithArgs(linker, dataDir, driveLetter)
			expectFormatterCalledWithArgs(formatter, diskNumber, partitionNumber)

			Expect(cmdRunner.RunCommands).To(ContainElement(Equal(
				strings.Split(addPartitionAccessPathCommand(diskNumber, partitionNumber), " "),
			)))
			Expect(cmdRunner.RunCommands).To(ContainElement(Equal(strings.Split(protectPathCmd(dataDir), " "))))
			Expect(partitioner.GetCountOnDiskCallCount()).To(Equal(0))
		})

		It("partitions an attached disk when disk is 1", func() {
			diskNumber = "1"
			partitionNumber = "1"

			partitioner.GetCountOnDiskReturns("0", nil)
			partitioner.PartitionDiskReturns(partitionNumber, nil)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(BeNumerically(">", 1))
			Expect(cmdRunner.RunCommands).To(Equal([][]string{
				strings.Split(addPartitionAccessPathCommand(diskNumber, partitionNumber), " "),
				strings.Split(getDriveLetter(diskNumber, partitionNumber), " "),
				strings.Split(protectPathCmd(dataDir), " "),
			}))

			Expect(partitioner.GetFreeSpaceOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetFreeSpaceOnDiskArgsForCall(0)).To(Equal(diskNumber))

			Expect(partitioner.InitializeDiskCallCount()).To(Equal(1))
			Expect(partitioner.InitializeDiskArgsForCall(0)).To(Equal(diskNumber))

			expectFormatterCalledWithArgs(formatter, diskNumber, partitionNumber)
			Expect(partitioner.GetCountOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetCountOnDiskArgsForCall(0)).To(Equal(diskNumber))

			Expect(partitioner.PartitionDiskCallCount()).To(Equal(1))
			Expect(partitioner.PartitionDiskArgsForCall(0)).To(Equal(diskNumber))
		})

		It("does nothing if partition exists on disk 0 and is linked to data dir", func() {
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.LinkTargetReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)
			partitioner.GetCountOnDiskReturns("1", nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(partitioner.PartitionDiskCallCount()).To(Equal(0))
		})

		It("doesn't initialize disk if a partition exists on disk 1", func() {
			diskNumber = "1"
			partitionNumber = "1"
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.LinkTargetReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)
			partitioner.GetCountOnDiskReturns("1", nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(partitioner.GetCountOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetCountOnDiskArgsForCall(0)).To(Equal(diskNumber))
			Expect(partitioner.InitializeDiskCallCount()).To(Equal(0))
		})

		It("doesn't warn about low disk space if partition exists and is linked to data dir", func() {
			partitioner.GetFreeSpaceOnDiskReturns(0, nil)
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.LinkTargetReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Consistently(logBuffer).ShouldNot(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(partitioner.PartitionDiskCallCount()).To(Equal(0))
		})

		It("logs a warning and doesn't create a partition if there is less than 1MB of free disk space", func() {
			partitioner.GetFreeSpaceOnDiskReturns((1024*1024)-1, nil)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).NotTo(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(partitioner.PartitionDiskCallCount()).To(Equal(0))
		})

		It("returns an error a warning when Protect-Path cmdlet is missing", func() {
			protector.CommandExistsReturns(false)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)
			Expect(err).To(MatchError(
				fmt.Sprintf("cannot protect %s. %s cmd does not exist.", dataDir, disk.ProtectCmdlet),
			))
		})

		It("returns an error when getting free disk space command fails", func() {
			expectedError := errors.New("It went wrong")
			partitioner.GetFreeSpaceOnDiskReturns(0, expectedError)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(expectedError))
		})

		It("returns an error when getting the count of existing partitions returns an error", func() {
			diskNumber = "1"
			partitionNumber = "1"

			partitionCountError := errors.New("Something failed")
			partitioner.GetCountOnDiskReturns("", partitionCountError)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(partitionCountError))
		})

		It("returns an error when Initialize-Disk command fails", func() {
			diskNumber = "1"
			partitionNumber = "1"

			cmdRunnerError := errors.New("It went wrong")
			partitioner.InitializeDiskReturns(cmdRunnerError)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(cmdRunnerError))
		})

		It("returns an error when partition disk command fails", func() {
			cmdRunnerError := errors.New("It went wrong")
			partitioner.PartitionDiskReturns("", cmdRunnerError)

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(cmdRunnerError))
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

		It("returns an error when Getting existing partition check command fails", func() {
			LinkError := errors.New("It went wrong")
			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			linker.LinkReturns(LinkError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(Equal(LinkError))
		})

		It("Returns an error when Protect-Path fails", func() {
			cmdStderr := protectPathError
			cmdRunner.AddCmdResult(protectPathCmd(dataDir), fakesys.FakeCmdResult{ExitStatus: 197, Stderr: cmdStderr})

			prepareSuccessfulFakeCommands(diskNumber, partitionNumber, dataDir, driveLetter)
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil)

			Expect(err).To(MatchError(
				fmt.Sprintf("Failed to protect path %s : %s", dataDir, cmdStderr),
			))
		})

		It("returns an error when calling protect-path command fails", func() {
			cmdRunnerError := errors.New("Failure")
			expandedCommand := protectPathCmd(dataDir)
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
				diskManager,
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

func protectPathCmd(dataDir string) string {
	removedTrailingSlash := strings.TrimRight(dataDir, "\\")
	return fmt.Sprintf(
		`powershell.exe Protect-Path '%s'`,
		removedTrailingSlash,
	)
}

func expectFormatterCalledWithArgs(
	formatter *fakedisk.FakeWindowsDiskFormatter,
	expectedDiskNumber,
	expectedPartitionNumber string,
) {

	ExpectWithOffset(1, formatter.FormatCallCount()).To(Equal(1))
	diskNumber, partitionNumber := formatter.FormatArgsForCall(0)
	ExpectWithOffset(1, diskNumber).To(Equal(expectedDiskNumber))
	ExpectWithOffset(1, partitionNumber).To(Equal(expectedPartitionNumber))
}

func expectLinkCalledWithArgs(linker *fakedisk.FakeWindowsDiskLinker, expectedLocation, expectedDriveLetter string) {
	Expect(linker.LinkCallCount()).To(Equal(1))
	linkLocation, linkTarget := linker.LinkArgsForCall(0)
	Expect(linkLocation).To(Equal(expectedLocation))
	Expect(linkTarget).To(Equal(fmt.Sprintf("%s:", expectedDriveLetter)))
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
				diskManager                = new(fakeplat.FakeWindowsDiskManager)
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
				diskManager,
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
				diskManager                = new(fakeplat.FakeWindowsDiskManager)
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
				diskManager,
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
