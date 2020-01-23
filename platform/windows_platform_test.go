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

	"github.com/google/uuid"

	"github.com/cloudfoundry/bosh-agent/platform/windows/powershell"

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
			err := platform.SetupDataDir(boshsettings.JobDir{})
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

			err := platform.SetupDataDir(boshsettings.JobDir{})
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

		const ExpPublicKey = "PUBLIC RSA KEY"

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

		It("reads the host RSA key", func() {
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
		var (
			diskNumber, partitionNumber, dataDir, driveLetter, labelPrefix string
		)

		BeforeEach(func() {
			labelPrefix = "fake-agent-id"
			diskNumber = "0"
			partitionNumber = "3"
			driveLetter = "E"
			dataDir = fmt.Sprintf(`C:%s\`, dirProvider.DataDir())

			partitioner.GetFreeSpaceOnDiskReturns(31404851200, nil)
			partitioner.PartitionDiskReturns(partitionNumber, nil)
			partitioner.AssignDriveLetterReturns(driveLetter, nil)

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

		It("does nothing when path is empty", func() {
			diskNumber = ""
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())
			Expect(diskManager.Invocations()).To(BeEmpty())
		})

		It("partitions the root disk when disk is 0", func() {
			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())

			Expect(protector.CommandExistsCallCount()).To(Equal(1))
			Expect(partitioner.GetCountOnDiskCallCount()).To(Equal(0))
			Expect(partitioner.InitializeDiskCallCount()).To(Equal(0))

			Expect(linker.LinkTargetCallCount()).To(Equal(1))
			Expect(linker.LinkTargetArgsForCall(0)).To(Equal(dataDir))

			Expect(partitioner.GetFreeSpaceOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetFreeSpaceOnDiskArgsForCall(0)).To(Equal(diskNumber))

			Expect(partitioner.PartitionDiskCallCount()).To(Equal(1))
			Expect(partitioner.PartitionDiskArgsForCall(0)).To(Equal(diskNumber))

			expectFormatterCalledWithArgs(formatter, diskNumber, partitionNumber)

			expectAssignDriveLetterCalledWithArgs(partitioner, diskNumber, partitionNumber)

			expectLinkCalledWithArgs(linker, dataDir, driveLetter)

			Expect(protector.ProtectPathCallCount()).To(Equal(1))
			Expect(protector.ProtectPathArgsForCall(0)).To(Equal(dataDir))
		})

		It("partitions an attached disk when disk is 1", func() {
			diskNumber = "1"
			partitionNumber = "1"

			partitioner.GetCountOnDiskReturns("0", nil)
			partitioner.PartitionDiskReturns(partitionNumber, nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())

			Expect(partitioner.GetCountOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetCountOnDiskArgsForCall(0)).To(Equal(diskNumber))

			Expect(partitioner.InitializeDiskCallCount()).To(Equal(1))
			Expect(partitioner.InitializeDiskArgsForCall(0)).To(Equal(diskNumber))
		})

		It("does nothing if partition exists on disk 0 and is linked to data dir", func() {
			linker.LinkTargetReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)
			partitioner.GetCountOnDiskReturns("1", nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())
			Expect(partitioner.PartitionDiskCallCount()).To(Equal(0))
		})

		It("doesn't initialize disk if a partition exists on disk 1", func() {
			diskNumber = "1"
			partitionNumber = "1"
			linker.LinkTargetReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)
			partitioner.GetCountOnDiskReturns("1", nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())
			Expect(partitioner.GetCountOnDiskCallCount()).To(Equal(1))
			Expect(partitioner.GetCountOnDiskArgsForCall(0)).To(Equal(diskNumber))
			Expect(partitioner.InitializeDiskCallCount()).To(Equal(0))
		})

		It("doesn't warn about low disk space if partition exists and is linked to data dir", func() {
			partitioner.GetFreeSpaceOnDiskReturns(0, nil)
			linker.LinkTargetReturns(fmt.Sprintf(`%s:\`, driveLetter), nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())
			Consistently(logBuffer).ShouldNot(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(partitioner.PartitionDiskCallCount()).To(Equal(0))
		})

		It("logs a warning and doesn't create a partition if there is less than 1MB of free disk space", func() {
			partitioner.GetFreeSpaceOnDiskReturns((1024*1024)-1, nil)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())
			Eventually(logBuffer).Should(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))
			Expect(partitioner.PartitionDiskCallCount()).To(Equal(0))
		})

		It("returns an error when Protect-Path cmdlet is missing", func() {
			protector.CommandExistsReturns(false)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)
			Expect(err).To(MatchError(
				fmt.Sprintf("cannot protect %s. %s cmd does not exist.", dataDir, disk.ProtectCmdlet),
			))
		})

		It("returns an error when getting free disk space command fails", func() {
			expectedError := errors.New("It went wrong")
			partitioner.GetFreeSpaceOnDiskReturns(0, expectedError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(expectedError))
		})

		It("returns an error when getting the count of existing partitions returns an error", func() {
			diskNumber = "1"
			partitionNumber = "1"

			partitionCountError := errors.New("Something failed")
			partitioner.GetCountOnDiskReturns("", partitionCountError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(partitionCountError))
		})

		It("returns an error when Initialize-Disk command fails", func() {
			diskNumber = "1"
			partitionNumber = "1"

			initializeDiskError := errors.New("It went wrong")
			partitioner.InitializeDiskReturns(initializeDiskError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(initializeDiskError))
		})

		It("returns an error when LinkTarget command fails", func() {
			linkTargetError := errors.New("failure")
			linker.LinkTargetReturns("", linkTargetError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(linkTargetError))
		})

		It("returns an error when partition disk command fails", func() {
			partitionDiskError := errors.New("It went wrong")
			partitioner.PartitionDiskReturns("", partitionDiskError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(partitionDiskError))
		})

		It("returns an error when format command fails", func() {
			formatError := errors.New("A failure occurred")
			formatter.FormatReturns(formatError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(formatError))
		})

		It("returns an error when attempting to assign a drive letter fails", func() {
			assignDriveLetterError := errors.New("failure")
			partitioner.AssignDriveLetterReturns("", assignDriveLetterError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(assignDriveLetterError))
		})

		It("returns an error when creating a symlink fails", func() {
			LinkError := errors.New("It went wrong")
			linker.LinkReturns(LinkError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(LinkError))
		})

		It("returns an error protecting path command fails", func() {
			protectPathError := errors.New("Failure")
			protector.ProtectPathReturns(protectPathError)

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).To(Equal(protectPathError))
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

			err := platform.SetupEphemeralDiskWithPath(diskNumber, nil, labelPrefix)

			Expect(err).NotTo(HaveOccurred())
			Consistently(logBuffer).ShouldNot(gbytes.Say(
				"WARN - Unable to create ephemeral partition on disk 0, as there isn't enough free space",
			))

			Expect(diskManager.Invocations()).To(BeEmpty())
		})
	})

	Describe("GetAgentSettingsPath", func() {
		It("logs that windows does not support tmpfs if asked for a tmpfs path", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "settings.json")

			path := platform.GetAgentSettingsPath(true)
			Expect(logBuffer).Should(gbytes.Say(
				"Windows does not support using tmpfs, using default agent settings path",
			))

			Expect(path).To(Equal(expectedPath))
		})

		It("returns the default settings path", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "settings.json")

			path := platform.GetAgentSettingsPath(false)
			Expect(path).To(Equal(expectedPath))
		})
	})

	Describe("GetPersistentDiskSettingsPath", func() {
		It("logs that windows does not support tmpfs if asked for a tmpfs path", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "persistent_disk_hints.json")

			path := platform.GetPersistentDiskSettingsPath(true)
			Expect(logBuffer).Should(gbytes.Say(
				"Windows does not support using tmpfs, using default persistent disk settings path",
			))

			Expect(path).To(Equal(expectedPath))
		})

		It("returns the default settings path", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "persistent_disk_hints.json")

			path := platform.GetPersistentDiskSettingsPath(false)
			Expect(path).To(Equal(expectedPath))
		})
	})
})

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

func expectAssignDriveLetterCalledWithArgs(
	partitioner *fakedisk.FakeWindowsDiskPartitioner,
	expectedDiskNumber,
	expectedPartitionNumber string,
) {

	ExpectWithOffset(1, partitioner.AssignDriveLetterCallCount()).To(Equal(1))
	diskNumber, partitionNumber := partitioner.AssignDriveLetterArgsForCall(0)
	Expect(diskNumber).To(Equal(expectedDiskNumber))
	Expect(partitionNumber).To(Equal(expectedPartitionNumber))
}

var _ = Describe("BOSH User Commands", func() {
	var (
		// We're doing this for real - no fakes!
		logger    = boshlog.NewLogger(boshlog.LevelNone)
		fs        = boshsys.NewOsFileSystem(logger)
		cmdRunner = boshsys.NewExecCmdRunner(logger)

		deleteUserOnce sync.Once

		testUsername string
	)

	BeforeEach(func() {
		testUsername = fmt.Sprintf("%stest_%s", boshsettings.EphemeralUserPrefix, fmt.Sprintf("%s", uuid.New())[0:8])

		deleteUserOnce.Do(func() {
			DeleteLocalUser(testUsername)
			DeleteLocalUser("vcap")
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
		DeleteLocalUser(testUsername)
		DeleteLocalUser("vcap")
		Expect(userExists(testUsername)).ToNot(Succeed())

		cmdRunner.RunCommand(powershell.Executable, "-Command", `get-wmiobject -class win32_userprofile | where { $_.LocalPath -like 'C:\Users\bosh*' } | remove-wmiobject`)
		cmdRunner.RunCommand(powershell.Executable, "-Command", fmt.Sprintf(`Remove-Item C:\Users\%s* -Force -Recurse`, testUsername))
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

		It("can create vcap user and insert authorized public keys into .ssh\\authorized_keys file", func() {
			if !sshdServiceIsInstalled() {
				Skip("This test requires the SSHD service to be installed")
			}

			keys := []string{
				"KEY_1",
				"KEY_2",
				"KEY_3",
			}
			Expect(userExists("vcap")).NotTo(Succeed())

			Expect(platform.SetupSSH(keys, "vcap")).To(Succeed())

			homedir, err := UserHomeDirectory("vcap")
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

		It("can delete a user, and any files in the user home directory which aren't in use by the registry", func() {
			Expect(platform.CreateUser(testUsername, "")).To(Succeed())
			Expect(userExists(testUsername)).To(Succeed())

			homedir, err := UserHomeDirectory(testUsername)
			Expect(err).To(Succeed())

			err = platform.SetupSSH([]string{"test-public-key"}, testUsername)
			Expect(err).NotTo(HaveOccurred())

			keyPath := filepath.Join(homedir, ".ssh", "authorized_keys")
			_, err = ioutil.ReadFile(keyPath)
			Expect(err).NotTo(HaveOccurred())

			userSID, _, _, err := cmdRunner.RunCommand(
				powershell.Executable,
				"-Command",
				fmt.Sprintf("(Get-LocalUser %s).SID.Value", testUsername),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(userSID).NotTo(BeEmpty())

			_, _, _, err = cmdRunner.RunCommand(
				"REG",
				"LOAD",
				fmt.Sprintf(`HKU\%s`, strings.TrimSpace(userSID)),
				fmt.Sprintf(`C:\Users\%s\NTUSER.dat`, testUsername),
			)
			Expect(err).NotTo(HaveOccurred())

			// Regex taken from: github.com/cloudfoundry/bosh-cli/director/ssh.go
			Expect(platform.DeleteEphemeralUsersMatching(fmt.Sprintf("^%s", testUsername))).To(Succeed())
			Expect(userExists(testUsername)).ToNot(Succeed())

			deletableUserProfileContents, _, _, err := cmdRunner.RunCommand(
				powershell.Executable,
				"-Command",
				fmt.Sprintf(`Get-ChildItem -force \Users\%s -exclude ntuser*`, testUsername),
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(deletableUserProfileContents).To(BeEmpty())

			_, _, _, err = cmdRunner.RunCommand(
				powershell.Executable,
				"-Command",
				fmt.Sprintf("Get-LocalUser %s", testUsername),
			)
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
