//go:build !windows
// +build !windows

package platform_test

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakeretry "github.com/cloudfoundry/bosh-utils/retrystrategy/fakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	fakeuuidgen "github.com/cloudfoundry/bosh-utils/uuid/fakes"

	fakelogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider/logstarproviderfakes"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	. "github.com/cloudfoundry/bosh-agent/v2/platform"
	fakecdrom "github.com/cloudfoundry/bosh-agent/v2/platform/cdrom/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/cert/certfakes"
	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	"github.com/cloudfoundry/bosh-agent/v2/platform/disk/diskfakes"
	fakedisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk/fakes"
	fakeplat "github.com/cloudfoundry/bosh-agent/v2/platform/fakes"
	fakenet "github.com/cloudfoundry/bosh-agent/v2/platform/net/fakes"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	fakestats "github.com/cloudfoundry/bosh-agent/v2/platform/stats/fakes"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager/servicemanagerfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

var _ = Describe("LinuxPlatform", func() {
	var (
		collector                  *fakestats.FakeCollector
		fs                         *fakesys.FakeFileSystem
		cmdRunner                  *fakesys.FakeCmdRunner
		diskManager                *diskfakes.FakeManager
		dirProvider                boshdirs.Provider
		devicePathResolver         *fakedpresolv.FakeDevicePathResolver
		platform                   Platform
		cdutil                     *fakecdrom.FakeCDUtil
		compressor                 boshcmd.Compressor
		copier                     boshcmd.Copier
		vitalsService              boshvitals.Service
		netManager                 *fakenet.FakeManager
		certManager                *certfakes.FakeManager
		monitRetryStrategy         *fakeretry.FakeRetryStrategy
		fakeDefaultNetworkResolver *fakenet.FakeDefaultNetworkResolver
		fakeAuditLogger            *fakeplat.FakeAuditLogger
		fakeLogsTarProvider        *fakelogstarprovider.FakeLogsTarProvider
		serviceManager             *servicemanagerfakes.FakeServiceManager

		fakeUUIDGenerator *fakeuuidgen.FakeGenerator

		state    *BootstrapState
		stateErr error
		options  LinuxOptions

		logger boshlog.Logger

		partitioner    *fakedisk.FakePartitioner
		formatter      *fakedisk.FakeFormatter
		mounter        *diskfakes.FakeMounter
		mountsSearcher *fakedisk.FakeMountsSearcher
		diskUtil       *fakedisk.FakeDiskUtil
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)

		collector = &fakestats.FakeCollector{}
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		dirProvider = boshdirs.NewProvider("/fake-dir")
		cdutil = fakecdrom.NewFakeCDUtil()
		compressor = boshcmd.NewTarballCompressor(cmdRunner, fs)
		copier = boshcmd.NewGenericCpCopier(fs, logger)
		netManager = &fakenet.FakeManager{}
		certManager = new(certfakes.FakeManager)
		monitRetryStrategy = fakeretry.NewFakeRetryStrategy()
		devicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		fakeDefaultNetworkResolver = &fakenet.FakeDefaultNetworkResolver{}
		serviceManager = &servicemanagerfakes.FakeServiceManager{}

		fakeUUIDGenerator = fakeuuidgen.NewFakeGenerator()
		fakeAuditLogger = fakeplat.NewFakeAuditLogger()

		fakeLogsTarProvider = &fakelogstarprovider.FakeLogsTarProvider{}

		state, stateErr = NewBootstrapState(fs, "/agent-state.json")
		Expect(stateErr).NotTo(HaveOccurred())

		options = LinuxOptions{}

		fs.SetGlob("/sys/bus/scsi/devices/*:0:0:0/block/*", []string{
			"/sys/bus/scsi/devices/0:0:0:0/block/sr0",
			"/sys/bus/scsi/devices/6:0:0:0/block/sdd",
			"/sys/bus/scsi/devices/fake-host-id:0:0:0/block/sda",
		})

		fs.SetGlob("/sys/bus/scsi/devices/fake-host-id:0:fake-disk-id:0/block/*", []string{
			"/sys/bus/scsi/devices/fake-host-id:0:fake-disk-id:0/block/sdf",
		})

		diskManager = &diskfakes.FakeManager{}
		partitioner = fakedisk.NewFakePartitioner()
		diskManager.GetEphemeralDevicePartitionerReturns(partitioner)
		diskManager.GetPersistentDevicePartitionerReturns(partitioner, nil)
		diskManager.GetRootDevicePartitionerReturns(partitioner)

		formatter = fakedisk.NewFakeFormatter()
		diskManager.GetFormatterReturns(formatter)

		mounter = &diskfakes.FakeMounter{}
		diskManager.GetMounterReturns(mounter)

		mountsSearcher = &fakedisk.FakeMountsSearcher{}
		diskManager.GetMountsSearcherReturns(mountsSearcher)

		diskUtil = fakedisk.NewFakeDiskUtil()
		diskManager.GetUtilReturns(diskUtil)

		vitalsService = boshvitals.NewService(collector, dirProvider, mounter)
	})

	JustBeforeEach(func() {
		platform = NewLinuxPlatform(
			fs,
			cmdRunner,
			collector,
			compressor,
			copier,
			dirProvider,
			vitalsService,
			cdutil,
			diskManager,
			netManager,
			certManager,
			monitRetryStrategy,
			devicePathResolver,
			state,
			options,
			logger,
			fakeDefaultNetworkResolver,
			fakeUUIDGenerator,
			fakeAuditLogger,
			fakeLogsTarProvider,
			serviceManager,
		)
	})

	Describe("SetupRuntimeConfiguration", func() {
		It("setups runtime configuration", func() {
			err := platform.SetupRuntimeConfiguration()
			Expect(err).NotTo(HaveOccurred())

			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"bosh-agent-rc"}))
		})
	})

	Describe("CreateUser", func() {
		It("creates user with an empty password", func() {
			fs.HomeDirHomePath = "/some/path/to/home1/foo-user"

			expectedUseradd := []string{
				"useradd",
				"-m",
				"-b", "/some/path/to/home1",
				"-s", "/bin/bash",
				"foo-user",
			}

			err := platform.CreateUser("foo-user", "/some/path/to/home1")
			Expect(err).NotTo(HaveOccurred())

			basePathStat := fs.GetFileTestStat("/some/path/to/home1")
			Expect(basePathStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(basePathStat.FileMode).To(Equal(os.FileMode(0755)))

			Expect(len(cmdRunner.RunCommands)).To(Equal(2))
			Expect(cmdRunner.RunCommands[0]).To(Equal(expectedUseradd))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"chmod", "700", "/some/path/to/home1/foo-user"}))
		})

		It("should handle errors when chmoding the home directory", func() {
			fs.HomeDirHomePath = "/some/path/to/home/foo-user"

			cmdRunner.AddCmdResult(
				"chmod 700 /some/path/to/home/foo-user",
				fakesys.FakeCmdResult{Error: errors.New("some error occurred"), Stdout: "error"},
			)

			err := platform.CreateUser("foo-user", "/some/path/to/home")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("AddUserToGroups", func() {
		It("adds user to groups", func() {
			err := platform.AddUserToGroups("foo-user", []string{"group1", "group2", "group3"})
			Expect(err).NotTo(HaveOccurred())

			Expect(len(cmdRunner.RunCommands)).To(Equal(1))

			usermod := []string{"usermod", "-G", "group1,group2,group3", "foo-user"}
			Expect(cmdRunner.RunCommands[0]).To(Equal(usermod))
		})
	})

	Describe("DeleteEphemeralUsersMatching", func() {
		It("deletes users with prefix and regex", func() {
			//nolint:gosec
			passwdFile := `bosh_foo:...
bosh_bar:...
foo:...
bar:...
foobar:...
bosh_foobar:...`

			err := fs.WriteFileString("/etc/passwd", passwdFile)
			Expect(err).NotTo(HaveOccurred())

			err = platform.DeleteEphemeralUsersMatching("bar$")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(2))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"userdel", "-rf", "bosh_bar"}))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"userdel", "-rf", "bosh_foobar"}))
		})
	})

	Describe("SetupBoshSettingsDisk", func() {
		It("mounts TmpFS in specified path", func() {
			err := platform.SetupBoshSettingsDisk()
			Expect(err).NotTo(HaveOccurred())

			Expect(fs.MkdirAllCallCount).To(Equal(1))

			Expect(mounter.MountTmpfsCallCount()).To(Equal(1))
			mountpoint, size := mounter.MountTmpfsArgsForCall(0)
			Expect(mountpoint).To(Equal("/fake-dir/bosh/settings"))
			Expect(size).To(Equal("16m"))
		})

		It("returns error when making directory fails", func() {
			fs.MkdirAllError = errors.New("my-bad-fake-err")

			err := platform.SetupBoshSettingsDisk()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("my-bad-fake-err"))
			Expect(err.Error()).To(ContainSubstring("Setting up Bosh Settings Disk"))
		})

		It("returns error if mounting tmpfs fails", func() {
			mounter.MountTmpfsReturns(errors.New("fake error"))

			err := platform.SetupBoshSettingsDisk()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake error"))
		})
	})

	Describe("SetupRootDisk", func() {
		var (
			labelPrefix string
		)

		BeforeEach(func() {
			labelPrefix = "fake-agent-id"

			mountsSearcher.SearchMountsMounts = []boshdisk.Mount{{
				PartitionPath: "/dev/sda1",
				MountPoint:    "/",
			}}

			devicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}
		})

		Context("when growpart is installed", func() {
			BeforeEach(func() {
				cmdRunner.CommandExistsValue = true
				options.CreatePartitionIfNoEphemeralDisk = false
			})

			It("runs growpart and resize2fs", func() {
				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda1",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda1"},
				)
				formatter.GetFileSystemType["/dev/sda1"] = "ext4"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(2))
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"growpart", "/dev/sda", "1"}))
				Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "resize2fs", Args: []string{"-f", "/dev/sda1"}}))
			})
			It("runs growpart and xfs_growfs", func() {
				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda1",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda1"},
				)
				formatter.GetFileSystemType["/dev/sda1"] = "xfs"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(2))
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"growpart", "/dev/sda", "1"}))
				Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "xfs_growfs", Args: []string{"-d", "/dev/sda1"}}))
			})

			It("runs growpart and resize2fs for the right root device number", func() {
				err := platform.SetupEphemeralDiskWithPath("/dev/sda", nil, labelPrefix)
				Expect(err).NotTo(HaveOccurred())

				mountsSearcher.SearchMountsMounts = []boshdisk.Mount{{
					PartitionPath: "/dev/sda2",
					MountPoint:    "/",
				}}

				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda2",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda2"},
				)

				formatter.GetFileSystemType["/dev/sda2"] = "ext4"
				err = platform.SetupRootDisk("/dev/sdb")

				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(3))
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"readlink", "-f", "/dev/sda2"}))
				Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"growpart", "/dev/sda", "2"}))
				Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "resize2fs", Args: []string{"-f", "/dev/sda2"}}))
			})

			It("runs growpart and xfs_growfs for the right root device number", func() {
				err := platform.SetupEphemeralDiskWithPath("/dev/sda", nil, labelPrefix)
				Expect(err).NotTo(HaveOccurred())

				mountsSearcher.SearchMountsMounts = []boshdisk.Mount{{
					PartitionPath: "/dev/sda2",
					MountPoint:    "/",
				}}

				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda2",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda2"},
				)

				formatter.GetFileSystemType["/dev/sda2"] = "xfs"
				err = platform.SetupRootDisk("/dev/sdb")

				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(3))
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"readlink", "-f", "/dev/sda2"}))
				Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"growpart", "/dev/sda", "2"}))
				Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "xfs_growfs", Args: []string{"-d", "/dev/sda2"}}))
			})

			It("returns error if it can't find the root device", func() {
				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda1",
					fakesys.FakeCmdResult{Error: errors.New("fake-readlink-error")},
				)

				formatter.GetFileSystemType["/dev/sda1"] = "ext4"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("findRootDevicePath: Shelling out to readlink: fake-readlink-error"))
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			})

			It("returns an error if growing the partiton fails", func() {
				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda1",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda1"},
				)

				cmdRunner.AddCmdResult(
					"growpart /dev/sda 1",
					fakesys.FakeCmdResult{Error: errors.New("fake-growpart-error")},
				)

				formatter.GetFileSystemType["/dev/sda1"] = "ext4"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("growpart: fake-growpart-error"))
				Expect(len(cmdRunner.RunCommands)).To(Equal(2))
			})

			It("returns error if resizing with 'resize2fs' on the filesystem fails", func() {
				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda1",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda1"},
				)

				cmdRunner.AddCmdResult(
					"resize2fs -f /dev/sda1",
					fakesys.FakeCmdResult{Error: errors.New("fake-resize2fs-error")},
				)

				formatter.GetFileSystemType["/dev/sda1"] = "ext4"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("resize2fs: fake-resize2fs-error"))
				Expect(len(cmdRunner.RunCommands)).To(Equal(2))
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
			})

			It("returns error if resizing with 'xfs_growfs' on the filesystem fails", func() {
				cmdRunner.AddCmdResult(
					"readlink -f /dev/sda1",
					fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/sda1"},
				)

				cmdRunner.AddCmdResult(
					"xfs_growfs -d /dev/sda1",
					fakesys.FakeCmdResult{Error: errors.New("fake-xfs_growfs-error")},
				)

				formatter.GetFileSystemType["/dev/sda1"] = "xfs"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("xfs_growfs: fake-xfs_growfs-error"))
				Expect(len(cmdRunner.RunCommands)).To(Equal(2))
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
			})

			It("skips growing root fs if no ephemerial disk is provided", func() {
				var platformWithNoEphemeralDisk Platform

				options.CreatePartitionIfNoEphemeralDisk = true
				platformWithNoEphemeralDisk = NewLinuxPlatform(
					fs,
					cmdRunner,
					collector,
					compressor,
					copier,
					dirProvider,
					vitalsService,
					cdutil,
					diskManager,
					netManager,
					certManager,
					monitRetryStrategy,
					devicePathResolver,
					state,
					options,
					logger,
					fakeDefaultNetworkResolver,
					fakeUUIDGenerator,
					fakeAuditLogger,
					fakeLogsTarProvider,
					serviceManager,
				)
				err := platformWithNoEphemeralDisk.SetupRootDisk("")

				Expect(err).ToNot(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(0))
			})
		})

		Context("when growpart is not installed", func() {
			BeforeEach(func() {
				cmdRunner.CommandExistsValue = false
				options.CreatePartitionIfNoEphemeralDisk = false
			})

			It("does not return error if growpart is not installed and skips growing fs", func() {
				formatter.GetFileSystemType["/dev/sda1"] = "ext4"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).ToNot(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(0))
			})
		})

		Context("when SkipDiskSetup is true", func() {
			BeforeEach(func() {
				options.SkipDiskSetup = true
				cmdRunner.CommandExistsValue = true
			})

			It("does nothing", func() {
				formatter.GetFileSystemType["/dev/sda1"] = "ext4"
				err := platform.SetupRootDisk("/dev/sdb")

				Expect(err).ToNot(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(0))
			})
		})

		Context("when disk is NVMe", func() {
			BeforeEach(func() {
				mountsSearcher.SearchMountsMounts = []boshdisk.Mount{{
					PartitionPath: "/dev/nvme0n1p1",
					MountPoint:    "/",
				}}

				devicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
					return diskSettings.Path, false, nil
				}
			})

			Context("when growpart is installed", func() {
				BeforeEach(func() {
					cmdRunner.CommandExistsValue = true
					options.CreatePartitionIfNoEphemeralDisk = false
				})

				It("runs growpart and resize2fs", func() {
					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p1"},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p1"] = "ext4"
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).NotTo(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
					Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"growpart", "/dev/nvme0n1", "1"}))
					Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "resize2fs", Args: []string{"-f", "/dev/nvme0n1p1"}}))
				})

				It("runs growpart and xfs_growfs", func() {
					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p1"},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p1"] = "xfs"
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).NotTo(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
					Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"growpart", "/dev/nvme0n1", "1"}))
					Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "xfs_growfs", Args: []string{"-d", "/dev/nvme0n1p1"}}))
				})

				It("runs growpart and resize2fs for the right root device number", func() {
					err := platform.SetupEphemeralDiskWithPath("/dev/nvme0n1", nil, labelPrefix)
					Expect(err).NotTo(HaveOccurred())

					mountsSearcher.SearchMountsMounts = []boshdisk.Mount{{
						PartitionPath: "/dev/nvme0n1p2",
						MountPoint:    "/",
					}}

					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p2",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p2"},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p2"] = "ext4"
					err = platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).NotTo(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(3))
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
					Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"readlink", "-f", "/dev/nvme0n1p2"}))
					Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"growpart", "/dev/nvme0n1", "2"}))
					Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "resize2fs", Args: []string{"-f", "/dev/nvme0n1p2"}}))
				})

				It("runs growpart and xfs_growfs for the right root device number", func() {
					err := platform.SetupEphemeralDiskWithPath("/dev/nvme0n1", nil, labelPrefix)
					Expect(err).NotTo(HaveOccurred())

					mountsSearcher.SearchMountsMounts = []boshdisk.Mount{{
						PartitionPath: "/dev/nvme0n1p2",
						MountPoint:    "/",
					}}

					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p2",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p2"},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p2"] = "xfs"
					err = platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).NotTo(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(3))
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
					Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"readlink", "-f", "/dev/nvme0n1p2"}))
					Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"growpart", "/dev/nvme0n1", "2"}))
					Expect(cmdRunner.RunComplexCommands[0]).To(Equal(boshsys.Command{Name: "xfs_growfs", Args: []string{"-d", "/dev/nvme0n1p2"}}))
				})

				It("returns error if it can't find the root device", func() {
					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: errors.New("fake-readlink-error")},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p1"] = "ext4"
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("findRootDevicePath: Shelling out to readlink: fake-readlink-error"))
					Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				})

				It("returns an error if growing the partiton fails", func() {
					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p1"},
					)

					cmdRunner.AddCmdResult(
						"growpart /dev/nvme0n1 1",
						fakesys.FakeCmdResult{Error: errors.New("fake-growpart-error")},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p1"] = "ext4"
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("growpart: fake-growpart-error"))
					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
				})

				It("returns error if resizing with 'resize2fs' on the filesystem fails", func() {
					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p1"},
					)

					cmdRunner.AddCmdResult(
						"resize2fs -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: errors.New("fake-resize2fs-error")},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p1"] = "ext4"
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("resize2fs: fake-resize2fs-error"))
					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
				})

				It("returns error if resizing with 'xfs_growfs' on the filesystem fails", func() {
					cmdRunner.AddCmdResult(
						"readlink -f /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: nil, Stdout: "/dev/nvme0n1p1"},
					)

					cmdRunner.AddCmdResult(
						"xfs_growfs -d /dev/nvme0n1p1",
						fakesys.FakeCmdResult{Error: errors.New("fake-xfs_growfs-error")},
					)

					formatter.GetFileSystemType["/dev/nvme0n1p1"] = "xfs"
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("xfs_growfs: fake-xfs_growfs-error"))
					Expect(len(cmdRunner.RunCommands)).To(Equal(2))
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(1))
				})

				It("skips growing root fs if no ephemerial disk is provided", func() {
					var platformWithNoEphemeralDisk Platform

					options.CreatePartitionIfNoEphemeralDisk = true
					platformWithNoEphemeralDisk = NewLinuxPlatform(
						fs,
						cmdRunner,
						collector,
						compressor,
						copier,
						dirProvider,
						vitalsService,
						cdutil,
						diskManager,
						netManager,
						certManager,
						monitRetryStrategy,
						devicePathResolver,
						state,
						options,
						logger,
						fakeDefaultNetworkResolver,
						fakeUUIDGenerator,
						fakeAuditLogger,
						fakeLogsTarProvider,
						serviceManager,
					)
					err := platformWithNoEphemeralDisk.SetupRootDisk("")

					Expect(err).ToNot(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(0))
				})
			})

			Context("when growpart is not installed", func() {
				BeforeEach(func() {
					cmdRunner.CommandExistsValue = false
					options.CreatePartitionIfNoEphemeralDisk = false
				})

				It("does not return error if growpart is not installed and skips growing fs", func() {
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).ToNot(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(0))
				})
			})

			Context("when SkipDiskSetup is true", func() {
				BeforeEach(func() {
					options.SkipDiskSetup = true
					cmdRunner.CommandExistsValue = true
				})

				It("does nothing", func() {
					err := platform.SetupRootDisk("/dev/nvme1n1")

					Expect(err).ToNot(HaveOccurred())
					Expect(len(cmdRunner.RunCommands)).To(Equal(0))
				})
			})
		})
	})

	Describe("SetupSSH", func() {
		It("setup ssh with a single key", func() {
			fs.HomeDirHomePath = "/some/home/dir"

			err := platform.SetupSSH([]string{"some public key"}, "vcap")
			Expect(err).NotTo(HaveOccurred())

			sshDirPath := "/some/home/dir/.ssh"
			sshDirStat := fs.GetFileTestStat(sshDirPath)

			Expect("vcap").To(Equal(fs.HomeDirUsername))

			Expect(sshDirStat).NotTo(BeNil())
			Expect(sshDirStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(os.FileMode(0700)).To(Equal(sshDirStat.FileMode))
			Expect("vcap").To(Equal(sshDirStat.Username))

			authKeysStat := fs.GetFileTestStat(path.Join(sshDirPath, "authorized_keys"))

			Expect(authKeysStat).NotTo(BeNil())
			Expect(fakesys.FakeFileTypeFile).To(Equal(authKeysStat.FileType))
			Expect(os.FileMode(0600)).To(Equal(authKeysStat.FileMode))
			Expect("vcap").To(Equal(authKeysStat.Username))
			Expect("some public key").To(Equal(authKeysStat.StringContents()))
		})

		It("setup ssh with multiple keys", func() {
			fs.HomeDirHomePath = "/some/home/dir"

			err := platform.SetupSSH([]string{"some public key", "some other public key"}, "vcap")
			Expect(err).NotTo(HaveOccurred())

			sshDirPath := "/some/home/dir/.ssh"
			sshDirStat := fs.GetFileTestStat(sshDirPath)

			Expect("vcap").To(Equal(fs.HomeDirUsername))

			Expect(sshDirStat).NotTo(BeNil())
			Expect(sshDirStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(os.FileMode(0700)).To(Equal(sshDirStat.FileMode))
			Expect("vcap").To(Equal(sshDirStat.Username))

			authKeysStat := fs.GetFileTestStat(path.Join(sshDirPath, "authorized_keys"))

			Expect(authKeysStat).NotTo(BeNil())
			Expect(fakesys.FakeFileTypeFile).To(Equal(authKeysStat.FileType))
			Expect(os.FileMode(0600)).To(Equal(authKeysStat.FileMode))
			Expect("vcap").To(Equal(authKeysStat.Username))
			Expect("some public key\nsome other public key").To(Equal(authKeysStat.StringContents()))
		})

	})

	Describe("SetUserPassword", func() {
		It("set user password", func() {
			err := platform.SetUserPassword("my-user", "my-encrypted-password")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"usermod", "-p", "my-encrypted-password", "my-user"}))
		})

		Context("password is empty string", func() {
			It("sets password to *", func() {
				err := platform.SetUserPassword("my-user", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"usermod", "-p", "*", "my-user"}))
			})
		})
	})

	Describe("SetupHostname", func() {
		const expectedEtcHosts = `127.0.0.1 foobar.local localhost

# The following lines are desirable for IPv6 capable hosts
::1 foobar.local localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
`
		Context("When running command to get hostname fails", func() {
			It("returns an error", func() {
				result := fakesys.FakeCmdResult{Error: errors.New("fake error")}
				cmdRunner.AddCmdResult("hostname foobar.local", result)

				err := platform.SetupHostname("foobar.local")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Setting hostname: fake error"))
			})
		})

		Context("When writing to the /etc/hostname file fails", func() {
			It("returns an error", func() {
				fs.WriteFileErrors["/etc/hostname"] = errors.New("ENXIO: disk failed")

				err := platform.SetupHostname("foobar.local")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Writing to /etc/hostname: ENXIO: disk failed"))
			})
		})

		Context("When writing to /etc/hosts file fails", func() {
			It("returns an error", func() {
				fs.WriteFileErrors["/etc/hosts"] = errors.New("ENXIO: disk failed")

				err := platform.SetupHostname("foobar.local")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Writing to /etc/hosts: ENXIO: disk failed"))
			})
		})

		Context("When saving bootstrap state fails", func() {
			It("returns an error", func() {
				fs.WriteFileErrors["/agent-state.json"] = errors.New("ENXIO: disk failed")

				err := platform.SetupHostname("foobar.local")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Setting up hostname: Writing bootstrap state to file: ENXIO: disk failed"))
			})
		})

		Context("When host files have not yet been configured", func() {
			It("sets up hostname", func() {
				err := platform.SetupHostname("foobar.local")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"hostname", "foobar.local"}))

				hostnameFileContent, err := fs.ReadFileString("/etc/hostname")
				Expect(err).NotTo(HaveOccurred())
				Expect(hostnameFileContent).To(Equal("foobar.local"))

				hostsFileContent, err := fs.ReadFileString("/etc/hosts")
				Expect(err).NotTo(HaveOccurred())
				Expect(hostsFileContent).To(Equal(expectedEtcHosts))
			})
		})

		Context("When host files have already been configured", func() {
			It("skips setting up hostname to prevent overriding changes made by the release author", func() {
				err := platform.SetupHostname("foobar.local")
				Expect(err).NotTo(HaveOccurred())
				err = platform.SetupHostname("newfoo.local")
				Expect(err).NotTo(HaveOccurred())

				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"hostname", "foobar.local"}))

				hostnameFileContent, err := fs.ReadFileString("/etc/hostname")
				Expect(err).NotTo(HaveOccurred())
				Expect(hostnameFileContent).To(Equal("foobar.local"))

				hostsFileContent, err := fs.ReadFileString("/etc/hosts")
				Expect(err).NotTo(HaveOccurred())
				Expect(hostsFileContent).To(Equal(expectedEtcHosts))
			})
		})
	})

	Describe("SetupLogrotate", func() {
		const expectedEtcLogrotate = `# Generated by bosh-agent

fake-base-path/data/sys/log/*.log fake-base-path/data/sys/log/.*.log fake-base-path/data/sys/log/*/*.log fake-base-path/data/sys/log/*/.*.log fake-base-path/data/sys/log/*/*/*.log fake-base-path/data/sys/log/*/*/.*.log {
  missingok
  rotate 7
  compress
  copytruncate
  size=fake-size
}
`

		It("sets up logrotate", func() {
			err := platform.SetupLogrotate("fake-group-name", "fake-base-path", "fake-size")
			Expect(err).NotTo(HaveOccurred())

			logrotateFileContent, err := fs.ReadFileString("/etc/logrotate.d/fake-group-name")
			Expect(err).NotTo(HaveOccurred())
			Expect(logrotateFileContent).To(Equal(expectedEtcLogrotate))

			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"/var/vcap/bosh/bin/setup-logrotate.sh"}))
		})
	})

	Describe("SetTimeWithNtpServers", func() {
		It("sets time with ntp servers", func() {
			err := platform.SetTimeWithNtpServers([]string{"0.north-america.pool.ntp.org", "1.north-america.pool.ntp.org"})
			Expect(err).NotTo(HaveOccurred())

			ntpConfig := fs.GetFileTestStat("/fake-dir/bosh/etc/ntpserver")
			Expect(ntpConfig.StringContents()).To(Equal("0.north-america.pool.ntp.org 1.north-america.pool.ntp.org"))
			Expect(ntpConfig.FileType).To(Equal(fakesys.FakeFileTypeFile))

			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"sync-time"}))
		})

		It("sets time with ntp servers is noop when no ntp server provided", func() {
			err := platform.SetTimeWithNtpServers([]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(0))

			ntpConfig := fs.GetFileTestStat("/fake-dir/bosh/etc/ntpserver")
			Expect(ntpConfig).To(BeNil())
		})
	})

	Describe("SetupEphemeralDiskWithPath", func() {
		var (
			labelPrefix         string
			expectedLabelPrefix string
		)

		BeforeEach(func() {
			labelPrefix = "fake-agent-id"
			expectedLabelPrefix = "bosh-partition-" + labelPrefix
		})

		itSetsUpEphemeralDisk := func(act func() error) {
			It("sets up ephemeral disk with path", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())

				dataDir := fs.GetFileTestStat("/fake-dir/data")
				Expect(dataDir.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(dataDir.FileMode).To(Equal(os.FileMode(0750)))
			})

			It("creates new partition even if the data directory is not empty", func() {
				fs.SetGlob(path.Join("/fake-dir", "data", "*"), []string{"something"})

				err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(partitioner.PartitionCalled).To(BeTrue())
				Expect(formatter.FormatCalled).To(BeTrue())
				Expect(mounter.MountCallCount()).To(Equal(1))
			})
		}

		Context("when ephemeral disk path is provided", func() {
			act := func() error {
				return platform.SetupEphemeralDiskWithPath("/dev/xvda", nil, labelPrefix)
			}

			itSetsUpEphemeralDisk(act)

			itTestsSetUpEphemeralDisk := func(act func() error, devicePath string) {
				partitionPath := func(devicePath string, paritionNumber int) string {
					switch {
					case strings.Contains(devicePath, "/dev/nvme"):
						return fmt.Sprintf("%sp%s", devicePath, strconv.Itoa(paritionNumber))
					default:
						return fmt.Sprintf("%s%s", devicePath, strconv.Itoa(paritionNumber))
					}
				}

				It("returns error if creating data dir fails", func() {
					fs.MkdirAllError = errors.New("fake-mkdir-all-err")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mkdir-all-err"))
					Expect(partitioner.PartitionCalled).To(BeFalse())
					Expect(formatter.FormatCalled).To(BeFalse())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})

				It("returns err when the data directory cannot be globbed", func() {
					fs.GlobErr = errors.New("fake-glob-err")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Globbing ephemeral disk mount point `/fake-dir/data/*'"))
					Expect(err.Error()).To(ContainSubstring("fake-glob-err"))
					Expect(partitioner.PartitionCalled).To(BeFalse())
					Expect(formatter.FormatCalled).To(BeFalse())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})

				It("returns err when mem stats are unavailable", func() {
					collector.MemStatsErr = errors.New("fake-memstats-error")
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Calculating partition sizes"))
					Expect(err.Error()).To(ContainSubstring("fake-memstats-error"))
					Expect(partitioner.PartitionCalled).To(BeFalse())
					Expect(formatter.FormatCalled).To(BeFalse())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})

				It("returns an error when partitioning fails", func() {
					partitioner.PartitionErr = errors.New("fake-partition-error")
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("Partitioning ephemeral disk '%s'", devicePath)))
					Expect(err.Error()).To(ContainSubstring("fake-partition-error"))
					Expect(formatter.FormatCalled).To(BeFalse())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})

				Context("for data partitions", func() {
					BeforeEach(func() {
						cmdRunner.AddCmdResult(
							"readlink -f /dev/xvda1",
							fakesys.FakeCmdResult{Stdout: "/dev/xvda1"},
						)

						cmdRunner.AddCmdResult(
							"readlink -f /dev/xvda2",
							fakesys.FakeCmdResult{Stdout: "/dev/xvda2"},
						)

						cmdRunner.AddCmdResult(
							"readlink -f /dev/nvme1n1p1",
							fakesys.FakeCmdResult{Stdout: "/dev/nvme1n1p1"},
						)

						cmdRunner.AddCmdResult(
							"readlink -f /dev/nvme1n1p2",
							fakesys.FakeCmdResult{Stdout: "/dev/nvme1n1p2"},
						)
					})

					It("formats swap and data partitions", func() {
						collector.MemStats.Total = uint64(1024 * 1024)
						partitioner.GetDeviceSizeInBytesSizes[devicePath] = uint64(1024 * 1024)
						err := act()
						Expect(err).NotTo(HaveOccurred())

						Expect(len(formatter.FormatPartitionPaths)).To(Equal(2))
						Expect(formatter.FormatPartitionPaths[0]).To(Equal(partitionPath(devicePath, 1)))
						Expect(formatter.FormatPartitionPaths[1]).To(Equal(partitionPath(devicePath, 2)))

						Expect(len(formatter.FormatFsTypes)).To(Equal(2))
						Expect(formatter.FormatFsTypes[0]).To(Equal(boshdisk.FileSystemSwap))
						Expect(formatter.FormatFsTypes[1]).To(Equal(boshdisk.FileSystemExt4))
					})

					It("mounts swap and data partitions", func() {
						collector.MemStats.Total = uint64(1024 * 1024)
						partitioner.GetDeviceSizeInBytesSizes[devicePath] = uint64(1024 * 1024)
						err := act()
						Expect(err).NotTo(HaveOccurred())

						Expect(mounter.MountCallCount()).To(Equal(1))
						partition, mntPoint, options := mounter.MountArgsForCall(0)
						Expect(partition).To(Equal(partitionPath(devicePath, 2)))
						Expect(mntPoint).To(Equal("/fake-dir/data"))
						Expect(options).To(BeEmpty())

						Expect(mounter.SwapOnCallCount()).To(Equal(1))
						partition = mounter.SwapOnArgsForCall(0)
						Expect(partition).To(Equal(partitionPath(devicePath, 1)))
					})
				})

				It("creates swap the size of the memory and the rest for data when disk is bigger than twice the memory", func() {
					memSizeInBytes := uint64(1024 * 1024 * 1024)
					diskSizeInBytes := 2*memSizeInBytes + 64
					fakePartitioner := partitioner
					fakePartitioner.GetDeviceSizeInBytesSizes[devicePath] = diskSizeInBytes
					collector.MemStats.Total = memSizeInBytes

					err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(fakePartitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
						{NamePrefix: expectedLabelPrefix, SizeInBytes: memSizeInBytes, Type: boshdisk.PartitionTypeSwap},
						{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes - memSizeInBytes, Type: boshdisk.PartitionTypeLinux},
					}))
				})

				It("creates equal swap and data partitions when disk is twice the memory or smaller", func() {
					memSizeInBytes := uint64(1024 * 1024 * 1024)
					diskSizeInBytes := 2*memSizeInBytes - 64
					fakePartitioner := partitioner
					fakePartitioner.GetDeviceSizeInBytesSizes[devicePath] = diskSizeInBytes
					collector.MemStats.Total = memSizeInBytes

					err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(fakePartitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
						{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes / 2, Type: boshdisk.PartitionTypeSwap},
						{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes / 2, Type: boshdisk.PartitionTypeLinux},
					}))
				})

				Context("when swap size is specified by user", func() {
					var diskSizeInBytes uint64 = 4096

					Context("and swap size is non-zero", func() {
						It("creates swap equal to specified amount", func() {
							var desiredSwapSize uint64 = 2048
							act = func() error {
								return platform.SetupEphemeralDiskWithPath(devicePath, &desiredSwapSize, labelPrefix)
							}
							partitioner.GetDeviceSizeInBytesSizes[devicePath] = diskSizeInBytes

							err := act()
							Expect(err).NotTo(HaveOccurred())
							Expect(partitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
								{NamePrefix: expectedLabelPrefix, SizeInBytes: 2048, Type: boshdisk.PartitionTypeSwap},
								{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes - 2048, Type: boshdisk.PartitionTypeLinux},
							}))

						})
					})

					Context("and swap size is zero", func() {
						It("does not attempt to create a swap disk", func() {
							cmdRunner.AddCmdResult(
								"readlink -f /dev/xvda1",
								fakesys.FakeCmdResult{Stdout: "/dev/xvda1"},
							)

							cmdRunner.AddCmdResult(
								"readlink -f /dev/nvme1n1p1",
								fakesys.FakeCmdResult{Stdout: "/dev/nvme1n1p1"},
							)

							var desiredSwapSize uint64
							act = func() error {
								return platform.SetupEphemeralDiskWithPath(devicePath, &desiredSwapSize, labelPrefix)
							}
							partitioner.GetDeviceSizeInBytesSizes[devicePath] = diskSizeInBytes

							err := act()
							Expect(err).NotTo(HaveOccurred())
							Expect(partitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
								{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes, Type: boshdisk.PartitionTypeLinux},
							}))

							Expect(formatter.FormatPartitionPaths).To(Equal([]string{partitionPath(devicePath, 1)}))
							Expect(mounter.SwapOnCallCount()).To(Equal(0))
						})
					})
				})

				Context("and swap size is not provided", func() {
					var diskSizeInBytes uint64 = 4096

					It("uses the default swap size options", func() {
						act = func() error {
							return platform.SetupEphemeralDiskWithPath(devicePath, nil, labelPrefix)
						}
						partitioner.GetDeviceSizeInBytesSizes[devicePath] = diskSizeInBytes
						collector.MemStats.Total = 2048

						err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(partitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
							{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes / 2, Type: boshdisk.PartitionTypeSwap},
							{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes / 2, Type: boshdisk.PartitionTypeLinux},
						}))
					})

					It("creates swap and data partitions when label prefix larger than 36", func() {
						labelPrefix = "12345678-1234-abcd-1234-1234abcd5678"
						expectedLabelPrefix = ("bosh-partition-" + labelPrefix)[0:32]
						act = func() error {
							return platform.SetupEphemeralDiskWithPath(devicePath, nil, labelPrefix)
						}
						partitioner.GetDeviceSizeInBytesSizes[devicePath] = diskSizeInBytes
						collector.MemStats.Total = 2048

						err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(partitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
							{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes / 2, Type: boshdisk.PartitionTypeSwap},
							{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes / 2, Type: boshdisk.PartitionTypeLinux},
						}))
					})
				})

			}

			itTestsSetUpEphemeralDisk(act, "/dev/xvda")

			Context("and is NVMe", func() {
				act = func() error {
					return platform.SetupEphemeralDiskWithPath("/dev/nvme1n1", nil, labelPrefix)
				}

				itSetsUpEphemeralDisk(act)

				itTestsSetUpEphemeralDisk(act, "/dev/nvme1n1")
			})
		})

		Context("when ephemeral disk path is not provided", func() {
			act := func() error {
				return platform.SetupEphemeralDiskWithPath("", nil, labelPrefix)
			}

			Context("when agent should partition ephemeral disk on root disk", func() {
				BeforeEach(func() {
					options.CreatePartitionIfNoEphemeralDisk = true
				})

				Context("when root device fails to be determined", func() {
					BeforeEach(func() {
						mountsSearcher.SearchMountsErr = errors.New("fake-mounts-searcher-error")
					})

					It("returns an error", func() {
						err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Finding root partition device"))
						Expect(partitioner.PartitionCalled).To(BeFalse())
						Expect(formatter.FormatCalled).To(BeFalse())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})

				Context("when root device is determined", func() {
					BeforeEach(func() {
						mountsSearcher.SearchMountsMounts = []boshdisk.Mount{
							{MountPoint: "/", PartitionPath: "rootfs"},
							{MountPoint: "/", PartitionPath: "/dev/vda1"},
						}
					})

					Context("when getting absolute path fails", func() {
						BeforeEach(func() {
							cmdRunner.AddCmdResult(
								"readlink -f /dev/vda1",
								fakesys.FakeCmdResult{Error: errors.New("fake-readlink-error")},
							)
						})

						It("returns an error", func() {
							err := act()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-readlink-error"))
							Expect(partitioner.PartitionCalled).To(BeFalse())
							Expect(formatter.FormatCalled).To(BeFalse())
							Expect(mounter.MountCallCount()).To(Equal(0))
						})
					})

					Context("when getting absolute path succeeds", func() {
						BeforeEach(func() {
							cmdRunner.AddCmdResult(
								"readlink -f /dev/vda1",
								fakesys.FakeCmdResult{Stdout: "/dev/vda1"},
							)
						})

						Context("when root device has insufficient space for ephemeral partitions", func() {
							BeforeEach(func() {
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = 1024*1024*1024 - 1
								collector.MemStats.Total = 8
							})

							It("returns an error", func() {
								err := act()
								Expect(err).To(HaveOccurred())
								Expect(err.Error()).To(ContainSubstring("Insufficient remaining disk"))
								Expect(partitioner.PartitionCalled).To(BeFalse())
								Expect(formatter.FormatCalled).To(BeFalse())
								Expect(mounter.MountCallCount()).To(Equal(0))
							})
						})

						Context("when root device has sufficient space for ephemeral partitions", func() {
							BeforeEach(func() {
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = 1024 * 1024 * 1024
								collector.MemStats.Total = 256 * 1024 * 1024

								cmdRunner.AddCmdResult(
									"readlink -f /dev/vda2",
									fakesys.FakeCmdResult{Stdout: "/dev/vda2"},
								)

								cmdRunner.AddCmdResult(
									"readlink -f /dev/vda3",
									fakesys.FakeCmdResult{Stdout: "/dev/vda3"},
								)
							})

							itSetsUpEphemeralDisk(act)

							It("returns err when mem stats are unavailable", func() {
								collector.MemStatsErr = errors.New("fake-memstats-error")
								err := act()
								Expect(err).To(HaveOccurred())
								Expect(err.Error()).To(ContainSubstring("Calculating partition sizes"))
								Expect(err.Error()).To(ContainSubstring("fake-memstats-error"))
								Expect(partitioner.PartitionCalled).To(BeFalse())
								Expect(formatter.FormatCalled).To(BeFalse())
								Expect(mounter.MountCallCount()).To(Equal(0))
							})

							It("returns an error when partitioning fails", func() {
								partitioner.PartitionErr = errors.New("fake-partition-error")
								err := act()
								Expect(err).To(HaveOccurred())
								Expect(err.Error()).To(ContainSubstring("Partitioning root device `/dev/vda'"))
								Expect(err.Error()).To(ContainSubstring("fake-partition-error"))
								Expect(formatter.FormatCalled).To(BeFalse())
								Expect(mounter.MountCallCount()).To(Equal(0))
							})

							It("formats swap and data partitions", func() {
								err := act()
								Expect(err).NotTo(HaveOccurred())

								Expect(len(formatter.FormatPartitionPaths)).To(Equal(2))
								Expect(formatter.FormatPartitionPaths[0]).To(Equal("/dev/vda2"))
								Expect(formatter.FormatPartitionPaths[1]).To(Equal("/dev/vda3"))

								Expect(len(formatter.FormatFsTypes)).To(Equal(2))
								Expect(formatter.FormatFsTypes[0]).To(Equal(boshdisk.FileSystemSwap))
								Expect(formatter.FormatFsTypes[1]).To(Equal(boshdisk.FileSystemExt4))
							})

							It("mounts swap and data partitions", func() {
								err := act()
								Expect(err).NotTo(HaveOccurred())

								Expect(mounter.MountCallCount()).To(Equal(1))
								partition, mntPoint, options := mounter.MountArgsForCall(0)
								Expect(partition).To(Equal("/dev/vda3"))
								Expect(mntPoint).To(Equal("/fake-dir/data"))
								Expect(options).To(BeEmpty())

								Expect(mounter.SwapOnCallCount()).To(Equal(1))
								partition = mounter.SwapOnArgsForCall(0)
								Expect(partition).To(Equal("/dev/vda2"))
							})

							It("creates swap the size of the memory and the rest for data when disk is bigger than twice the memory", func() {
								memSizeInBytes := uint64(1024 * 1024 * 1024)
								diskSizeInBytes := 2*memSizeInBytes + 64
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = diskSizeInBytes
								collector.MemStats.Total = memSizeInBytes

								err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(partitioner.PartitionDevicePath).To(Equal("/dev/vda"))
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: memSizeInBytes,
										Type:        boshdisk.PartitionTypeSwap,
									}),
								)
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: diskSizeInBytes - memSizeInBytes,
										Type:        boshdisk.PartitionTypeLinux,
									}),
								)
							})

							It("creates equal swap and data partitions when disk is twice the memory or smaller", func() {
								memSizeInBytes := uint64(1024 * 1024 * 1024)
								diskSizeInBytes := 2*memSizeInBytes - 64
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = diskSizeInBytes
								collector.MemStats.Total = memSizeInBytes

								err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(partitioner.PartitionDevicePath).To(Equal("/dev/vda"))
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: diskSizeInBytes / 2,
										Type:        boshdisk.PartitionTypeSwap,
									}),
								)
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: diskSizeInBytes / 2,
										Type:        boshdisk.PartitionTypeLinux,
									}),
								)
							})

							Context("when swap size is specified by user", func() {
								diskSizeInBytes := 2 * uint64(1024*1024*1024)

								Context("and swap size is non-zero", func() {
									It("creates swap equal to specified amount", func() {
										var desiredSwapSize uint64 = 2048
										act := func() error {
											return platform.SetupEphemeralDiskWithPath("", &desiredSwapSize, labelPrefix)
										}
										partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = diskSizeInBytes

										err := act()
										Expect(err).NotTo(HaveOccurred())
										Expect(partitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
											{NamePrefix: expectedLabelPrefix, SizeInBytes: 2048, Type: boshdisk.PartitionTypeSwap},
											{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes - 2048, Type: boshdisk.PartitionTypeLinux},
										}))

									})
								})

								Context("and swap size is zero", func() {
									It("does not attempt to create a swap disk", func() {
										cmdRunner.AddCmdResult(
											"readlink -f /dev/vda2",
											fakesys.FakeCmdResult{Stdout: "/dev/vda2"},
										)

										var desiredSwapSize uint64
										act := func() error {
											return platform.SetupEphemeralDiskWithPath("", &desiredSwapSize, labelPrefix)
										}
										partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = diskSizeInBytes

										err := act()
										Expect(err).NotTo(HaveOccurred())
										Expect(partitioner.PartitionPartitions).To(Equal([]boshdisk.Partition{
											{NamePrefix: expectedLabelPrefix, SizeInBytes: diskSizeInBytes, Type: boshdisk.PartitionTypeLinux},
										}))

										Expect(formatter.FormatPartitionPaths).To(Equal([]string{"/dev/vda2"}))
										Expect(mounter.SwapOnCallCount()).To(Equal(0))
									})
								})
							})
						})

						Context("when getting root device remaining size fails", func() {
							BeforeEach(func() {
								partitioner.GetDeviceSizeInBytesErr = errors.New("fake-get-remaining-size-error")
							})

							It("returns an error", func() {
								err := act()
								Expect(err).To(HaveOccurred())
								Expect(err.Error()).To(ContainSubstring("Getting root device remaining size"))
								Expect(err.Error()).To(ContainSubstring("fake-get-remaining-size-error"))
								Expect(partitioner.PartitionCalled).To(BeFalse())
								Expect(formatter.FormatCalled).To(BeFalse())
								Expect(mounter.MountCallCount()).To(Equal(0))
							})
						})
					})
				})

				Context("when root device is determined and root partition is not the first one", func() {
					BeforeEach(func() {
						mountsSearcher.SearchMountsMounts = []boshdisk.Mount{
							{MountPoint: "/boot", PartitionPath: "/dev/vda1"},
							{MountPoint: "/", PartitionPath: "rootfs"},
							{MountPoint: "/", PartitionPath: "/dev/vda2"},
						}
					})

					Context("when getting absolute path succeeds", func() {
						BeforeEach(func() {
							cmdRunner.AddCmdResult(
								"readlink -f /dev/vda2",
								fakesys.FakeCmdResult{Stdout: "/dev/vda2"},
							)
						})

						Context("when root device has sufficient space for ephemeral partitions", func() {
							BeforeEach(func() {
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = 1024 * 1024 * 1024
								collector.MemStats.Total = 256 * 1024 * 1024

								cmdRunner.AddCmdResult(
									"readlink -f /dev/vda3",
									fakesys.FakeCmdResult{Stdout: "/dev/vda3"},
								)

								cmdRunner.AddCmdResult(
									"readlink -f /dev/vda4",
									fakesys.FakeCmdResult{Stdout: "/dev/vda4"},
								)

							})

							itSetsUpEphemeralDisk(act)

							It("formats swap and data partitions", func() {
								err := act()
								Expect(err).NotTo(HaveOccurred())

								Expect(len(formatter.FormatPartitionPaths)).To(Equal(2))
								Expect(formatter.FormatPartitionPaths[0]).To(Equal("/dev/vda3"))
								Expect(formatter.FormatPartitionPaths[1]).To(Equal("/dev/vda4"))

								Expect(len(formatter.FormatFsTypes)).To(Equal(2))
								Expect(formatter.FormatFsTypes[0]).To(Equal(boshdisk.FileSystemSwap))
								Expect(formatter.FormatFsTypes[1]).To(Equal(boshdisk.FileSystemExt4))
							})

							It("mounts swap and data partitions", func() {
								err := act()
								Expect(err).NotTo(HaveOccurred())

								Expect(mounter.MountCallCount()).To(Equal(1))
								partition, mntPoint, options := mounter.MountArgsForCall(0)
								Expect(partition).To(Equal("/dev/vda4"))
								Expect(mntPoint).To(Equal("/fake-dir/data"))
								Expect(options).To(BeEmpty())

								Expect(mounter.SwapOnCallCount()).To(Equal(1))
								partition = mounter.SwapOnArgsForCall(0)
								Expect(partition).To(Equal("/dev/vda3"))
							})

							It("creates swap the size of the memory and the rest for data when disk is bigger than twice the memory", func() {
								memSizeInBytes := uint64(1024 * 1024 * 1024)
								diskSizeInBytes := 2*memSizeInBytes + 64
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = diskSizeInBytes
								collector.MemStats.Total = memSizeInBytes

								err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(partitioner.PartitionDevicePath).To(Equal("/dev/vda"))
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: memSizeInBytes,
										Type:        boshdisk.PartitionTypeSwap,
									}),
								)
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: diskSizeInBytes - memSizeInBytes,
										Type:        boshdisk.PartitionTypeLinux,
									}),
								)
							})

							It("creates equal swap and data partitions when disk is twice the memory or smaller", func() {
								memSizeInBytes := uint64(1024 * 1024 * 1024)
								diskSizeInBytes := 2*memSizeInBytes - 64
								partitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = diskSizeInBytes
								collector.MemStats.Total = memSizeInBytes

								err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(partitioner.PartitionDevicePath).To(Equal("/dev/vda"))
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: diskSizeInBytes / 2,
										Type:        boshdisk.PartitionTypeSwap,
									}),
								)
								Expect(partitioner.PartitionPartitions).To(ContainElement(
									boshdisk.Partition{
										NamePrefix:  expectedLabelPrefix,
										SizeInBytes: diskSizeInBytes / 2,
										Type:        boshdisk.PartitionTypeLinux,
									}),
								)
							})
						})
					})
				})

				It("returns error if creating data dir fails", func() {
					fs.MkdirAllError = errors.New("fake-mkdir-all-err")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mkdir-all-err"))
					Expect(partitioner.PartitionCalled).To(BeFalse())
					Expect(formatter.FormatCalled).To(BeFalse())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})

				It("returns err when the data directory cannot be globbed", func() {
					fs.GlobErr = errors.New("fake-glob-err")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Globbing ephemeral disk mount point `/fake-dir/data/*'"))
					Expect(err.Error()).To(ContainSubstring("fake-glob-err"))
					Expect(partitioner.PartitionCalled).To(BeFalse())
					Expect(formatter.FormatCalled).To(BeFalse())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})

			Context("when agent should not partition ephemeral disk on root disk", func() {
				BeforeEach(func() {
					options.CreatePartitionIfNoEphemeralDisk = false
				})

				It("returns an error", func() {
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot use root partition as ephemeral disk"))
				})

				It("does not try to partition anything", func() {
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(partitioner.PartitionCalled).To(BeFalse())
				})

				It("does not try to format anything", func() {
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(formatter.FormatCalled).To(BeFalse())
				})

				It("does not try to mount anything", func() {
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})
		})

		Context("when SkipDiskSetup is true", func() {
			BeforeEach(func() {
				options.SkipDiskSetup = true
			})

			It("makes sure ephemeral directory is there but does nothing else", func() {
				swapSize := uint64(0)
				err := platform.SetupEphemeralDiskWithPath("/dev/xvda", &swapSize, labelPrefix)
				Expect(err).ToNot(HaveOccurred())

				dataDir := fs.GetFileTestStat("/fake-dir/data")
				Expect(dataDir.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(dataDir.FileMode).To(Equal(os.FileMode(0750)))

				Expect(partitioner.PartitionCalled).To(BeFalse())
				Expect(formatter.FormatCalled).To(BeFalse())
				Expect(mounter.MountCallCount()).To(Equal(0))
			})
		})
	})

	Describe("SetupRawEphemeralDisks", func() {
		It("labels the raw ephemeral paths for unpartitioned disks", func() {
			result := fakesys.FakeCmdResult{
				Error:      nil,
				ExitStatus: 0,
				Stderr:     "",
				Stdout: `Model: Xen Virtual Block Device (xvd)
Disk /dev/xvdb: 40.3GB
Sector size (logical/physical): 512B/512B
Partition Table: loop

Number  Start  End     Size    File system  Flags
1      0.00B  40.3GB  40.3GB  ext3
`,
			}

			cmdRunner.AddCmdResult("parted -s /dev/xvdb p", result)

			result = fakesys.FakeCmdResult{
				Error:      nil,
				ExitStatus: 0,
				Stderr:     "",
				Stdout: `Model: Xen Virtual Block Device (xvd)
Disk /dev/xvdc: 40.3GB
Sector size (logical/physical): 512B/512B
Partition Table: loop

Number  Start  End     Size    File system  Flags
1      0.00B  40.3GB  40.3GB  ext3
`,
			}

			cmdRunner.AddCmdResult("parted -s /dev/xvdc p", result)

			devicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}

			err := platform.SetupRawEphemeralDisks([]boshsettings.DiskSettings{{Path: "/dev/xvdb"}, {Path: "/dev/xvdc"}})

			Expect(err).ToNot(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(4))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"parted", "-s", "/dev/xvdb", "p"}))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"parted", "-s", "/dev/xvdb", "mklabel", "gpt", "unit", "%", "mkpart", "raw-ephemeral-0", "0", "100"}))
			Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"parted", "-s", "/dev/xvdc", "p"}))
			Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"parted", "-s", "/dev/xvdc", "mklabel", "gpt", "unit", "%", "mkpart", "raw-ephemeral-1", "0", "100"}))
		})

		It("does not label the raw ephemeral paths for already partitioned disks", func() {
			result := fakesys.FakeCmdResult{
				Error:      nil,
				ExitStatus: 0,
				Stderr:     "",
				Stdout: `Model: Xen Virtual Block Device (xvd)
Disk /dev/xvdb: 40.3GB
Sector size (logical/physical): 512B/512B
Partition Table: gpt

Number  Start   End     Size    File system  Name             Flags
 1      1049kB  40.3GB  40.3GB               raw-ephemeral-0
`,
			}

			cmdRunner.AddCmdResult("parted -s /dev/xvdb p", result)

			result = fakesys.FakeCmdResult{
				Error:      nil,
				ExitStatus: 0,
				Stderr:     "",
				Stdout: `Model: Xen Virtual Block Device (xvd)
Disk /dev/xvdc: 40.3GB
Sector size (logical/physical): 512B/512B
Partition Table: gpt

Number  Start   End     Size    File system  Name             Flags
 1      1049kB  40.3GB  40.3GB               raw-ephemeral-1
`,
			}

			cmdRunner.AddCmdResult("parted -s /dev/xvdc p", result)

			devicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}

			err := platform.SetupRawEphemeralDisks([]boshsettings.DiskSettings{{Path: "/dev/xvdb"}, {Path: "/dev/xvdc"}})

			Expect(err).ToNot(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(2))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"parted", "-s", "/dev/xvdb", "p"}))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"parted", "-s", "/dev/xvdc", "p"}))
		})

		It("does not give an error if parted prints 'unrecognised disk label' to stdout and returns an error", func() {
			result := fakesys.FakeCmdResult{
				Error:      errors.New("fake-parted-error"),
				ExitStatus: 0,
				Stderr:     "",
				Stdout: `Model: Xen Virtual Block Device (xvd)
Error: /dev/xvda: unrecognised disk label
Disk /dev/xvda: 40.3GB
Sector size (logical/physical): 512B/512B
Partition Table: gpt

Number  Start   End     Size    File system  Name             Flags
 1      1049kB  40.3GB  40.3GB               raw-ephemeral-0
`,
			}

			cmdRunner.AddCmdResult("parted -s /dev/xvda p", result)

			devicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}

			err := platform.SetupRawEphemeralDisks([]boshsettings.DiskSettings{{Path: "/dev/xvda"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"parted", "-s", "/dev/xvda", "p"}))
		})

		It("does not give an error if parted prints 'unrecognised disk label' to stderr and returns an error", func() {
			result := fakesys.FakeCmdResult{
				Error:      errors.New("fake-parted-error"),
				ExitStatus: 0,
				Stderr:     "Error: /dev/xvda: unrecognised disk label",
				Stdout: `Model: Xen Virtual Block Device (xvd)
Disk /dev/xvda: 40.3GB
Sector size (logical/physical): 512B/512B
Partition Table: gpt

Number  Start   End     Size    File system  Name             Flags
 1      1049kB  40.3GB  40.3GB               raw-ephemeral-0
`,
			}

			cmdRunner.AddCmdResult("parted -s /dev/xvda p", result)

			devicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}

			err := platform.SetupRawEphemeralDisks([]boshsettings.DiskSettings{{Path: "/dev/xvda"}})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"parted", "-s", "/dev/xvda", "p"}))
		})

		Context("when SkipDiskSetup is true", func() {
			BeforeEach(func() {
				options.SkipDiskSetup = true
			})

			It("does nothing", func() {
				err := platform.SetupRawEphemeralDisks([]boshsettings.DiskSettings{{Path: "/dev/xvdb"}, {Path: "/dev/xvdc"}})

				Expect(err).ToNot(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(0))
			})
		})
	})

	Describe("SetupDataDir", func() {
		It("creates jobs directory in data directory", func() {
			err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
			Expect(err).NotTo(HaveOccurred())

			sysLogStats := fs.GetFileTestStat("/fake-dir/data/jobs")
			Expect(sysLogStats).ToNot(BeNil())
			Expect(sysLogStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(sysLogStats.FileMode).To(Equal(os.FileMode(0750)))
			Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/jobs"}))
		})

		It("creates sensitive blobs directory in data directory", func() {
			err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
			Expect(err).NotTo(HaveOccurred())

			sysLogStats := fs.GetFileTestStat("/fake-dir/data/sensitive_blobs")
			Expect(sysLogStats).ToNot(BeNil())
			Expect(sysLogStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(sysLogStats.FileMode).To(Equal(os.FileMode(0700)))
			Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/sensitive_blobs"}))
		})

		It("creates packages directory in data directory", func() {
			err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
			Expect(err).NotTo(HaveOccurred())

			sysLogStats := fs.GetFileTestStat("/fake-dir/data/packages")
			Expect(sysLogStats).ToNot(BeNil())
			Expect(sysLogStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(sysLogStats.FileMode).To(Equal(os.FileMode(0755)))
			Expect(cmdRunner.RunCommands[4]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/packages"}))
		})

		Context("when a tmpfs for the sensitive directories is requested", func() {
			It("mounts that with the default size", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{
					TmpFS: true,
				}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.MountTmpfsCallCount()).To(Equal(3))

				mountPoint, size := mounter.MountTmpfsArgsForCall(0)
				Expect(mountPoint).To(Equal("/fake-dir/data/jobs"))
				Expect(size).To(Equal("100m"))

				mountPoint, size = mounter.MountTmpfsArgsForCall(1)
				Expect(mountPoint).To(Equal("/fake-dir/data/sensitive_blobs"))
				Expect(size).To(Equal("100m"))
			})

			It("can use a non-default value", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{
					TmpFS:     true,
					TmpFSSize: "42m",
				}, boshsettings.RunDir{
					TmpFSSize: "24m",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.MountTmpfsCallCount()).To(Equal(3))

				mountPoint, size := mounter.MountTmpfsArgsForCall(0)
				Expect(mountPoint).To(Equal("/fake-dir/data/jobs"))
				Expect(size).To(Equal("42m"))

				mountPoint, size = mounter.MountTmpfsArgsForCall(1)
				Expect(mountPoint).To(Equal("/fake-dir/data/sensitive_blobs"))
				Expect(size).To(Equal("42m"))

				mountPoint, size = mounter.MountTmpfsArgsForCall(2)
				Expect(mountPoint).To(Equal("/fake-dir/data/sys/run"))
				Expect(size).To(Equal("24m"))

			})
		})

		Context("when sys/run is already mounted", func() {
			BeforeEach(func() {
				mounter.IsMountPointReturns("", true, nil)
			})

			It("creates sys/log directory in data directory", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				sysLogStats := fs.GetFileTestStat("/fake-dir/data/sys/log")
				Expect(sysLogStats).ToNot(BeNil())
				Expect(sysLogStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(sysLogStats.FileMode).To(Equal(os.FileMode(0750)))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/sys"}))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/sys/log"}))
			})

			It("creates symlink from sys to data/sys", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				sysStats := fs.GetFileTestStat("/fake-dir/sys")
				Expect(sysStats).ToNot(BeNil())
				Expect(sysStats.FileType).To(Equal(fakesys.FakeFileTypeSymlink))
				Expect(sysStats.SymlinkTarget).To(Equal("/fake-dir/data/sys"))
			})

			It("does not create new sys/run dir", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				sysRunStats := fs.GetFileTestStat("/fake-dir/data/sys/run")
				Expect(sysRunStats).To(BeNil())
			})

			It("does not mount tmpfs again", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())
				Expect(mounter.MountCallCount()).To(Equal(0))
			})
		})

		Context("when bosh/canrestart is not yet mounted", func() {
			BeforeEach(func() {
				mounter.IsMountPointReturns("", false, nil)
			})

			It("creates new bosh/canrestart dir", func() {
				err := platform.SetupCanRestartDir()
				Expect(err).NotTo(HaveOccurred())

				sysRunStats := fs.GetFileTestStat("/fake-dir/bosh/canrestart")
				Expect(sysRunStats).ToNot(BeNil())
				Expect(sysRunStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(sysRunStats.FileMode).To(Equal(os.FileMode(0740)))
				Expect(cmdRunner.RunCommands).To(HaveLen(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/bosh/canrestart"}))
			})

			It("returns an error if chowning the canRestartDir fails", func() {
				fakeResult := fakesys.FakeCmdResult{Error: errors.New("some error occurred")}
				cmdRunner.AddCmdResult(
					"chown root:vcap /fake-dir/bosh/canrestart",
					fakeResult,
				)
				err := platform.SetupCanRestartDir()
				Expect(err).To(MatchError("Chowning canrestart dir: some error occurred"))
			})

			It("mounts tmpfs to bosh/canrestart", func() {
				err := platform.SetupCanRestartDir()
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.MountTmpfsCallCount()).To(Equal(1))
				mountpoint, size := mounter.MountTmpfsArgsForCall(0)
				Expect(mountpoint).To(Equal("/fake-dir/bosh/canrestart"))
				Expect(size).To(Equal("16m"))
			})

			It("returns an error if creation of mount point fails", func() {
				fs.MkdirAllError = errors.New("fake-mkdir-error")

				err := platform.SetupCanRestartDir()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
			})

			It("returns an error if mounting tmpfs fails", func() {
				mounter.MountTmpfsReturns(errors.New("fake-mount-error"))

				err := platform.SetupCanRestartDir()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-mount-error"))
			})
		})

		Context("when sys/run is not yet mounted", func() {
			BeforeEach(func() {
				mounter.IsMountPointReturns("", false, nil)
			})

			It("creates sys/log directory in data directory", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				sysLogStats := fs.GetFileTestStat("/fake-dir/data/sys/log")
				Expect(sysLogStats).ToNot(BeNil())
				Expect(sysLogStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(sysLogStats.FileMode).To(Equal(os.FileMode(0750)))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/sys"}))
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/sys/log"}))
			})

			It("creates symlink from sys to data/sys", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				sysStats := fs.GetFileTestStat("/fake-dir/sys")
				Expect(sysStats).ToNot(BeNil())
				Expect(sysStats.FileType).To(Equal(fakesys.FakeFileTypeSymlink))
				Expect(sysStats.SymlinkTarget).To(Equal("/fake-dir/data/sys"))
			})

			It("creates new sys/run dir", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				sysRunStats := fs.GetFileTestStat("/fake-dir/data/sys/run")
				Expect(sysRunStats).ToNot(BeNil())
				Expect(sysRunStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(sysRunStats.FileMode).To(Equal(os.FileMode(0750)))
				Expect(cmdRunner.RunCommands[5]).To(Equal([]string{"chown", "root:vcap", "/fake-dir/data/sys/run"}))
			})

			It("mounts tmpfs to sys/run", func() {
				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.MountTmpfsCallCount()).To(Equal(1))
				mntPt, size := mounter.MountTmpfsArgsForCall(0)
				Expect(mntPt).To(Equal("/fake-dir/data/sys/run"))
				Expect(size).To(Equal("16m"))
			})

			It("returns an error if creation of mount point fails", func() {
				fs.MkdirAllError = errors.New("fake-mkdir-error")

				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
			})

			It("returns an error if mounting tmpfs fails", func() {
				mounter.MountTmpfsReturns(errors.New("fake-mount-error"))

				err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-mount-error"))
			})
		})
	})

	Describe("SetupHomeDir", func() {
		act := func() error {
			return platform.SetupHomeDir()
		}

		Context("/home is not mounted", func() {
			It("mounts the /home dir", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.MountFilesystemCallCount()).To(Equal(1))
				partition, mntPt, fstype, options := mounter.MountFilesystemArgsForCall(0)
				Expect(partition).To(Equal("/home"))
				Expect(mntPt).To(Equal("/home"))
				Expect(fstype).To(Equal(""))
				Expect(options).To(Equal([]string{"bind"}))

				Expect(mounter.RemountInPlaceCallCount()).To(Equal(1))
				mntPt, options = mounter.RemountInPlaceArgsForCall(0)
				Expect(mntPt).To(Equal("/home"))
				Expect(options).To(Equal([]string{"nodev", "nosuid"}))
			})

			It("return error if it cannot mount", func() {
				mounter.MountFilesystemReturns(errors.New("fake-mount-error"))
				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-mount-error"))
				Expect(mounter.RemountInPlaceCallCount()).To(Equal(0))
			})

			It("return error if it cannot remount in place", func() {
				mounter.RemountInPlaceReturns(errors.New("fake-remount-error"))
				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-remount-error"))
				Expect(mounter.MountFilesystemCallCount()).To(Equal(1))
			})
		})

		Context("/home is mounted", func() {
			BeforeEach(func() {
				mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
					if devicePathOrMountPoint == "/home" {
						return true, nil
					}
					return false, nil
				}
			})
			It("does no op", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(mounter.MountCallCount()).To(Equal(0))
				Expect(mounter.RemountInPlaceCallCount()).To(Equal(0))
			})
		})
	})

	Describe("SetupTmpDir", func() {
		var originalTMPDir string

		BeforeEach(func() {
			originalTMPDir = os.Getenv("TMPDIR")
		})

		AfterEach(func() {
			Expect(os.Setenv("TMPDIR", originalTMPDir)).To(Succeed())
		})

		It("changes permissions on /tmp", func() {
			err := platform.SetupTmpDir()
			Expect(err).NotTo(HaveOccurred())

			Expect(cmdRunner.RunCommands).To(ContainElement([]string{"chown", "root:vcap", "/var/tmp"}))
			Expect(cmdRunner.RunCommands).To(ContainElement([]string{"chmod", "1777", "/var/tmp"}))
		})

		It("creates new temp dir", func() {
			err := platform.SetupTmpDir()
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat("/fake-dir/data/tmp")
			Expect(fileStats).NotTo(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(fileStats.FileMode).To(Equal(os.FileMode(0755)))
		})

		It("returns error if creating new temp dir errs", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-error")

			err := platform.SetupTmpDir()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
		})

		It("sets TMPDIR environment variable so that children of this process will use new temp dir", func() {
			err := platform.SetupTmpDir()
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Getenv("TMPDIR")).To(Equal("/fake-dir/data/tmp"))
		})

		Context("when UseDefaultTmpDir option is set to false", func() {
			BeforeEach(func() {
				options.UseDefaultTmpDir = false
			})

			It("creates a root_tmp folder", func() {
				err := platform.SetupTmpDir()
				Expect(err).NotTo(HaveOccurred())
				Expect(cmdRunner.RunCommands).To(ContainElement([]string{"mkdir", "-p", "/fake-dir/data/root_tmp"}))
			})

			It("changes permissions on the new bind mount folder", func() {
				err := platform.SetupTmpDir()
				Expect(err).NotTo(HaveOccurred())

				Expect(cmdRunner.RunCommands).To(ContainElement([]string{"chmod", "1777", "/fake-dir/data/root_tmp"}))
			})

			Context("mounting root_tmp into /tmp", func() {
				Context("when /tmp is not a mount point", func() {
					BeforeEach(func() {
						mounter.IsMountPointReturns("", false, nil)
					})

					It("bind mounts it in /tmp", func() {
						err := platform.SetupTmpDir()
						Expect(err).NotTo(HaveOccurred())

						Expect(mounter.MountFilesystemCallCount()).To(Equal(2))
						partition, mntPt, fstype, options := mounter.MountFilesystemArgsForCall(0)
						Expect(partition).To(Equal("/fake-dir/data/root_tmp"))
						Expect(mntPt).To(Equal("/tmp"))
						Expect(fstype).To(Equal(""))
						Expect(options).To(Equal([]string{"bind"}))

						Expect(mounter.RemountInPlaceCallCount()).To(Equal(2))
						mntPt, options = mounter.RemountInPlaceArgsForCall(0)
						Expect(mntPt).To(Equal("/tmp"))
						Expect(options).To(Equal([]string{"nodev", "nosuid", "noexec"}))
					})
				})

				Context("when /tmp is a mount point", func() {
					BeforeEach(func() {
						mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
							if devicePathOrMountPoint == "/tmp" {
								return true, nil
							}
							return false, nil
						}
					})

					Context("when remount fails", func() {
						BeforeEach(func() {
							mounter.RemountInPlaceReturns(errors.New("remount error"))
						})

						It("returns an error", func() {
							err := platform.SetupTmpDir()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(Equal("remount error"))
						})
					})

					It("returns without an error", func() {
						err := platform.SetupTmpDir()
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal("/tmp"))
						Expect(err).ToNot(HaveOccurred())

						Expect(mounter.RemountInPlaceCallCount()).To(Equal(2))
						mntPt, options := mounter.RemountInPlaceArgsForCall(0)
						Expect(mntPt).To(Equal("/tmp"))
						Expect(options).To(Equal([]string{"nodev", "nosuid", "noexec"}))
					})

					It("does not create new tmp filesystem", func() {
						err := platform.SetupTmpDir()
						Expect(err).NotTo(HaveOccurred())
						for _, cmd := range cmdRunner.RunCommands {
							Expect(cmd[0]).ToNot(Equal("truncate"))
							Expect(cmd[0]).ToNot(Equal("mke2fs"))
						}
					})

					It("does not try to mount root_tmp into /tmp", func() {
						Expect(platform.SetupTmpDir()).To(Succeed())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})

				Context("when /tmp cannot be determined if it is a mount point", func() {
					BeforeEach(func() {
						mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
							if devicePathOrMountPoint == "/tmp" {
								return false, errors.New("fake-is-mounted-error")
							}
							return false, nil
						}
					})

					It("returns error", func() {
						err := platform.SetupTmpDir()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-is-mounted-error"))
					})

					It("does not create new tmp filesystem", func() {
						err := platform.SetupTmpDir()
						Expect(err).To(HaveOccurred())
						for _, cmd := range cmdRunner.RunCommands {
							Expect(cmd[0]).ToNot(Equal("truncate"))
							Expect(cmd[0]).ToNot(Equal("mke2fs"))
						}
					})

					It("does not try to mount /tmp", func() {
						err := platform.SetupTmpDir()
						Expect(err).To(HaveOccurred())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})
			})

			Context("mounting root_tmp into /var/tmp", func() {
				Context("when /var/tmp is not a mount point", func() {
					BeforeEach(func() {
						mounter.IsMountedReturns(false, nil)
					})

					It("bind mounts it in /var/tmp", func() {
						err := platform.SetupTmpDir()
						Expect(err).NotTo(HaveOccurred())

						Expect(mounter.MountFilesystemCallCount()).To(Equal(2))
						partition, mntPt, fstype, options := mounter.MountFilesystemArgsForCall(1)
						Expect(partition).To(Equal("/fake-dir/data/root_tmp"))
						Expect(mntPt).To(Equal("/var/tmp"))
						Expect(fstype).To(Equal(""))
						Expect(options).To(ConsistOf("bind"))

						Expect(mounter.RemountInPlaceCallCount()).To(Equal(2))
						mntPt, options = mounter.RemountInPlaceArgsForCall(1)
						Expect(mntPt).To(Equal("/var/tmp"))
						Expect(options).To(Equal([]string{"nodev", "nosuid", "noexec"}))
					})

					It("changes permissions for the system /var/tmp folder", func() {
						err := platform.SetupTmpDir()
						Expect(err).NotTo(HaveOccurred())

						Expect(cmdRunner.RunCommands).To(ContainElement([]string{"chown", "root:vcap", "/var/tmp"}))
					})
				})

				Context("when /var/tmp is a mount point", func() {
					BeforeEach(func() {
						mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
							if devicePathOrMountPoint == "/var/tmp" {
								return true, nil
							}
							return false, nil
						}
					})

					It("returns without an error", func() {
						err := platform.SetupTmpDir()
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal("/tmp"))
						Expect(mounter.IsMountedArgsForCall(1)).To(Equal("/var/tmp"))
						Expect(err).ToNot(HaveOccurred())
					})

					It("does not create new tmp filesystem", func() {
						err := platform.SetupTmpDir()
						Expect(err).NotTo(HaveOccurred())
						for _, cmd := range cmdRunner.RunCommands {
							Expect(cmd[0]).ToNot(Equal("truncate"))
							Expect(cmd[0]).ToNot(Equal("mke2fs"))
						}
					})

					It("does not try to mount root_tmp into /var/tmp", func() {
						err := platform.SetupTmpDir()
						Expect(err).NotTo(HaveOccurred())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})

				Context("when /var/tmp cannot be determined if it is a mount point", func() {
					BeforeEach(func() {
						mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
							if devicePathOrMountPoint == "/var/tmp" {
								return false, errors.New("fake-is-mounted-error")
							}
							return false, nil
						}
					})

					It("returns error", func() {
						err := platform.SetupTmpDir()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-is-mounted-error"))
					})

					It("does not create new tmp filesystem", func() {
						err := platform.SetupTmpDir()
						Expect(err).To(HaveOccurred())
						for _, cmd := range cmdRunner.RunCommands {
							Expect(cmd[0]).ToNot(Equal("truncate"))
							Expect(cmd[0]).ToNot(Equal("mke2fs"))
						}
					})

					It("does not try to mount /var/tmp", func() {
						err := platform.SetupTmpDir()
						Expect(err).To(HaveOccurred())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})
			})

			Context("mounting after agent has been restarted", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/tmp" {
							return true, nil
						}
						return false, nil
					}
				})

				It("mounts unmounted tmp dirs", func() {
					err := platform.SetupTmpDir()
					Expect(err).ToNot(HaveOccurred())
					Expect(mounter.MountFilesystemCallCount()).To(Equal(1))
					_, mntPt, _, _ := mounter.MountFilesystemArgsForCall(0)
					Expect(mntPt).To(Equal("/var/tmp"))
				})
			})
		})

		Context("when UseDefaultTmpDir option is set to true", func() {
			BeforeEach(func() {
				options.UseDefaultTmpDir = true
			})

			It("returns without an error", func() {
				err := platform.SetupTmpDir()
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not create new tmp filesystem", func() {
				err := platform.SetupTmpDir()
				Expect(err).NotTo(HaveOccurred())
				for _, cmd := range cmdRunner.RunCommands {
					Expect(cmd[0]).ToNot(Equal("truncate"))
					Expect(cmd[0]).ToNot(Equal("mke2fs"))
				}
			})

			It("does not try to mount anything", func() {
				err := platform.SetupTmpDir()
				Expect(err).NotTo(HaveOccurred())
				Expect(mounter.MountCallCount()).To(Equal(0))
			})
		})
	})

	Describe("SetupSharedMemory", func() {
		Context("when /dev/shm exists as a mount", func() {
			BeforeEach(func() {
				mounter.IsMountPointStub = func(path string) (string, bool, error) {
					if path == "/dev/shm" {
						return "something", true, nil
					}

					return "", false, nil
				}
			})

			It("remounts /dev/shm with security flags", func() {
				err := platform.SetupSharedMemory()
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.IsMountPointCallCount()).To(Equal(2))
				Expect(mounter.IsMountPointArgsForCall(0)).To(Equal("/dev/shm"))
				Expect(mounter.IsMountPointArgsForCall(1)).To(Equal("/run/shm"))

				Expect(mounter.RemountInPlaceCallCount()).To(Equal(1))
				mntPt, options := mounter.RemountInPlaceArgsForCall(0)
				Expect(mntPt).To(Equal("/dev/shm"))
				Expect(options).To(Equal([]string{"noexec", "nodev", "nosuid"}))
			})

			Context("when remounting /dev/shm fails", func() {
				BeforeEach(func() {
					mounter.RemountInPlaceReturns(errors.New("boom"))
				})

				It("returns an error", func() {
					err := platform.SetupSharedMemory()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when /run/shm exists as a mount", func() {
			BeforeEach(func() {
				mounter.IsMountPointStub = func(path string) (string, bool, error) {
					if path == "/run/shm" {
						return "something", true, nil
					}

					return "", false, nil
				}
			})

			It("remounts /run/shm with security flags", func() {
				err := platform.SetupSharedMemory()
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.IsMountPointCallCount()).To(Equal(2))
				Expect(mounter.IsMountPointArgsForCall(0)).To(Equal("/dev/shm"))
				Expect(mounter.IsMountPointArgsForCall(1)).To(Equal("/run/shm"))

				Expect(mounter.RemountInPlaceCallCount()).To(Equal(1))
				mntPt, options := mounter.RemountInPlaceArgsForCall(0)
				Expect(mntPt).To(Equal("/run/shm"))
				Expect(options).To(Equal([]string{"noexec", "nodev", "nosuid"}))
			})

			Context("when remounting /run/shm fails", func() {
				BeforeEach(func() {
					mounter.RemountInPlaceReturns(errors.New("boom"))
				})

				It("returns an error", func() {
					err := platform.SetupSharedMemory()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when neither /dev/shm or /run/shm exist as a mount point", func() {
			BeforeEach(func() {
				mounter.IsMountPointReturns("", false, nil)
			})

			It("does not remount anything", func() {
				err := platform.SetupSharedMemory()
				Expect(err).NotTo(HaveOccurred())

				Expect(mounter.IsMountPointCallCount()).To(Equal(2))
				Expect(mounter.IsMountPointArgsForCall(0)).To(Equal("/dev/shm"))
				Expect(mounter.IsMountPointArgsForCall(1)).To(Equal("/run/shm"))

				Expect(mounter.RemountInPlaceCallCount()).To(Equal(0))
			})
		})

		Context("when detecting the mount point fails", func() {
			BeforeEach(func() {
				mounter.IsMountPointReturns("", false, errors.New("boom"))
			})

			It("returns an error", func() {
				err := platform.SetupSharedMemory()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("SetupLogDir", func() {
		act := func() error {
			return platform.SetupLogDir()
		}

		Context("invariant log setup", func() {
			It("creates a root_log folder with permissions", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())
				testFileStat := fs.GetFileTestStat("/fake-dir/data/root_log")
				Expect(testFileStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"chmod", "0771", "/fake-dir/data/root_log"}))
			})

			It("creates an audit dir in root_log folder and changes its permissions", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"mkdir", "-p", "/fake-dir/data/root_log/audit"}))
				Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"chmod", "0750", "/fake-dir/data/root_log/audit"}))
			})

			It("creates an sysstat dir in root_log folder and changes permissions", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"mkdir", "-p", "/fake-dir/data/root_log/sysstat"}))
				Expect(cmdRunner.RunCommands[4]).To(Equal([]string{"chmod", "0755", "/fake-dir/data/root_log/sysstat"}))
			})

			It("changes ownership on the new bind mount folder after all that", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())

				Expect(cmdRunner.RunCommands[5]).To(Equal([]string{"chown", "root:syslog", "/fake-dir/data/root_log"}))
			})

			It("touches, chmods and chowns wtmp, btmp and lastlog  files", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())

				Expect(cmdRunner.RunCommands[6]).To(Equal([]string{"touch", "/fake-dir/data/root_log/btmp"}))
				Expect(cmdRunner.RunCommands[7]).To(Equal([]string{"chown", "root:utmp", "/fake-dir/data/root_log/btmp"}))
				Expect(cmdRunner.RunCommands[8]).To(Equal([]string{"chmod", "0600", "/fake-dir/data/root_log/btmp"}))

				Expect(cmdRunner.RunCommands[9]).To(Equal([]string{"touch", "/fake-dir/data/root_log/wtmp"}))
				Expect(cmdRunner.RunCommands[10]).To(Equal([]string{"chown", "root:utmp", "/fake-dir/data/root_log/wtmp"}))
				Expect(cmdRunner.RunCommands[11]).To(Equal([]string{"chmod", "0664", "/fake-dir/data/root_log/wtmp"}))

				Expect(cmdRunner.RunCommands[12]).To(Equal([]string{"touch", "/fake-dir/data/root_log/lastlog"}))
				Expect(cmdRunner.RunCommands[13]).To(Equal([]string{"chown", "root:utmp", "/fake-dir/data/root_log/lastlog"}))
				Expect(cmdRunner.RunCommands[14]).To(Equal([]string{"chmod", "0664", "/fake-dir/data/root_log/lastlog"}))
			})
		})

		Context("chrony log setup", func() {
			It("does not create, chmod, or chown the /var/log/chrony directory", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())

				Expect(cmdRunner.RunCommands).ToNot(ContainElement([]string{"mkdir", "-p", "/fake-dir/data/root_log/chrony"}))
				Expect(cmdRunner.RunCommands).ToNot(ContainElement([]string{"chmod", "0700", "/fake-dir/data/root_log/chrony"}))
				Expect(cmdRunner.RunCommands).ToNot(ContainElement([]string{"chown", "_chrony:_chrony", "/fake-dir/data/root_log/chrony"}))
			})

			Context("when the chrony user exists", func() {
				BeforeEach(func() {
					err := fs.WriteFileString("/etc/passwd", `bob:fakeuser
_chrony:somethingfake
sam:fakeanotheruser`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("creates the /var/log/chrony directory", func() {
					err := act()
					Expect(err).NotTo(HaveOccurred())

					Expect(cmdRunner.RunCommands[15]).To(Equal([]string{"mkdir", "-p", "/fake-dir/data/root_log/chrony"}))
					Expect(cmdRunner.RunCommands[16]).To(Equal([]string{"chmod", "0700", "/fake-dir/data/root_log/chrony"}))
					Expect(cmdRunner.RunCommands[17]).To(Equal([]string{"chown", "_chrony:_chrony", "/fake-dir/data/root_log/chrony"}))
				})
			})

			Context("when there is an error reading /etc/passwd", func() {
				It("acts like there is no chrony user", func() {
					err := fs.WriteFileString("/etc/passwd", `_notchrony:somethingfake`)
					Expect(err).NotTo(HaveOccurred())
					fs.RegisterReadFileError("/etc/passwd", fmt.Errorf("boom"))

					err = act()
					Expect(err).ToNot(HaveOccurred())

					Expect(cmdRunner.RunCommands).ToNot(ContainElement([]string{"mkdir", "-p", "/fake-dir/data/root_log/chrony"}))
					Expect(cmdRunner.RunCommands).ToNot(ContainElement([]string{"chmod", "0700", "/fake-dir/data/root_log/chrony"}))
					Expect(cmdRunner.RunCommands).ToNot(ContainElement([]string{"chown", "_chrony:_chrony", "/fake-dir/data/root_log/chrony"}))
				})
			})
		})

		Context("mounting root_log into /var/log", func() {
			Context("when /var/log is not a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", false, nil)
				})

				It("bind mounts it in /var/log", func() {
					err := act()
					Expect(err).NotTo(HaveOccurred())

					Expect(mounter.MountFilesystemCallCount()).To(Equal(1))
					partition, mntPt, fstype, options := mounter.MountFilesystemArgsForCall(0)
					Expect(partition).To(Equal("/fake-dir/data/root_log"))
					Expect(mntPt).To(Equal("/var/log"))
					Expect(fstype).To(Equal(""))
					Expect(options).To(Equal([]string{"bind"}))
				})
			})

			Context("when /var/log is a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/var/log" {
							return true, nil
						}
						return false, nil
					}
				})

				It("returns without an error", func() {
					err := act()
					Expect(mounter.IsMountedArgsForCall(0)).To(Equal("/var/log"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not try to mount root_log into /var/log", func() {
					err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})

			Context("when /var/log cannot be determined if it is a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/var/log" {
							return false, errors.New("fake-is-mounted-error")
						}
						return false, nil
					}
				})

				It("returns error", func() {
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-mounted-error"))
				})

				It("does not try to mount /var/log", func() {
					err := act()
					Expect(err).To(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("SetupOptDir", func() {
		It("creates a root_var_opt folder with permissions", func() {
			err := platform.SetupOptDir()
			Expect(err).NotTo(HaveOccurred())
			testFileStat := fs.GetFileTestStat("/fake-dir/data/root_var_opt")
			Expect(testFileStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(testFileStat.FileMode).To(Equal(os.FileMode(0755)))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"chown", "root:root", "/fake-dir/data/root_var_opt"}))
		})

		It("creates a root_opt folder with permissions", func() {
			err := platform.SetupOptDir()
			Expect(err).NotTo(HaveOccurred())
			testFileStat := fs.GetFileTestStat("/fake-dir/data/root_opt")
			Expect(testFileStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(testFileStat.FileMode).To(Equal(os.FileMode(0755)))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"chown", "root:root", "/fake-dir/data/root_opt"}))
		})

		Context("mounting root_var_opt into /var/opt", func() {
			Context("when /var/opt is not a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", false, nil)
				})

				It("bind mounts it in /var/opt", func() {
					err := platform.SetupOptDir()
					Expect(err).NotTo(HaveOccurred())

					partition, mntPt, fstype, options := mounter.MountFilesystemArgsForCall(0)
					Expect(partition).To(Equal("/fake-dir/data/root_var_opt"))
					Expect(mntPt).To(Equal("/var/opt"))
					Expect(fstype).To(Equal(""))
					Expect(options).To(Equal([]string{"bind"}))

					mntPt, options = mounter.RemountInPlaceArgsForCall(0)
					Expect(mntPt).To(Equal("/var/opt"))
					Expect(options).To(Equal([]string{"nodev"}))
				})
			})

			Context("when /var/opt is a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/var/opt" {
							return true, nil
						}
						return false, nil
					}
				})

				It("returns without an error", func() {
					err := platform.SetupOptDir()
					Expect(mounter.IsMountedArgsForCall(0)).To(Equal("/var/opt"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not try to mount root_var_opt into /var/opt", func() {
					err := platform.SetupOptDir()
					Expect(err).NotTo(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})

			Context("when /var/opt cannot be determined if it is a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/var/opt" {
							return false, errors.New("fake-is-mounted-error")
						}
						return false, nil
					}
				})

				It("returns error", func() {
					err := platform.SetupOptDir()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-mounted-error"))
				})

				It("does not try to mount /var/opt", func() {
					err := platform.SetupOptDir()
					Expect(err).To(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})
		})

		Context("mounting root_opt into /opt", func() {
			Context("when /opt is not a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", false, nil)
				})

				It("bind mounts it in /opt", func() {
					err := platform.SetupOptDir()
					Expect(err).NotTo(HaveOccurred())

					partition, mntPt, fstype, options := mounter.MountFilesystemArgsForCall(1)
					Expect(partition).To(Equal("/fake-dir/data/root_opt"))
					Expect(mntPt).To(Equal("/opt"))
					Expect(fstype).To(Equal(""))
					Expect(options).To(Equal([]string{"bind"}))

					mntPt, options = mounter.RemountInPlaceArgsForCall(1)
					Expect(mntPt).To(Equal("/opt"))
					Expect(options).To(Equal([]string{"nodev"}))
				})
			})

			Context("when /opt is a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/opt" {
							return true, nil
						}
						return false, nil
					}
				})

				It("returns without an error", func() {
					err := platform.SetupOptDir()
					Expect(mounter.IsMountedArgsForCall(1)).To(Equal("/opt"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not try to mount root_opt into /opt", func() {
					err := platform.SetupOptDir()
					Expect(err).NotTo(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})

			Context("when /opt cannot be determined if it is a mount point", func() {
				BeforeEach(func() {
					mounter.IsMountedStub = func(devicePathOrMountPoint string) (bool, error) {
						if devicePathOrMountPoint == "/opt" {
							return false, errors.New("fake-is-mounted-error")
						}
						return false, nil
					}
				})

				It("returns error", func() {
					err := platform.SetupOptDir()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-mounted-error"))
				})

				It("does not try to mount /opt", func() {
					err := platform.SetupOptDir()
					Expect(err).To(HaveOccurred())
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})
		})
	})

	Describe("SetupBlobDir", func() {
		act := func() error {
			return platform.SetupBlobsDir()
		}

		It("creates a blobs folder with correct permissions", func() {
			err := act()
			Expect(err).NotTo(HaveOccurred())
			testFileStat := fs.GetFileTestStat("/fake-dir/data/blobs")
			Expect(testFileStat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(testFileStat.FileMode).To(Equal(os.FileMode(0700)))
		})

		It("creates a blobs folder with correct ownership", func() {
			err := act()
			Expect(err).NotTo(HaveOccurred())
			Expect(cmdRunner.RunCommands).To(ContainElement([]string{"chown", "root:vcap", "/fake-dir/data/blobs"}))
		})
	})

	Describe("SetupLoggingAndAuditing", func() {
		act := func() error {
			return platform.SetupLoggingAndAuditing()
		}

		Context("when logging and auditing startup script runs successfully", func() {
			BeforeEach(func() {
				fakeResult := fakesys.FakeCmdResult{Error: nil}
				cmdRunner.AddCmdResult("/var/vcap/bosh/bin/bosh-start-logging-and-auditing", fakeResult)
			})

			It("returns no error", func() {
				err := act()

				Expect(err).NotTo(HaveOccurred())
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"/var/vcap/bosh/bin/bosh-start-logging-and-auditing"}))
			})
		})

		Context("when logging and auditing startup script runs successfully", func() {
			BeforeEach(func() {
				fakeResult := fakesys.FakeCmdResult{Error: errors.New("FAIL")}
				cmdRunner.AddCmdResult("/var/vcap/bosh/bin/bosh-start-logging-and-auditing", fakeResult)
			})

			It("returns an error", func() {
				err := act()

				Expect(err).To(HaveOccurred())
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"/var/vcap/bosh/bin/bosh-start-logging-and-auditing"}))
			})
		})
	})

	Describe("AdjustPersistentDiskPartitioning", func() {
		var (
			diskSettings boshsettings.DiskSettings
			mntPoint     string
		)

		BeforeEach(func() {
			diskSettings = boshsettings.DiskSettings{
				ID:           "fake-unique-id",
				Path:         "fake-volume-id",
				MountOptions: []string{"mntOpt1", "mntOpt2"},
			}
			mntPoint = "/mnt/point"
		})

		Context("when UsePreformattedPersistentDisk set to true", func() {
			BeforeEach(func() {
				options.UsePreformattedPersistentDisk = true
			})

			It("does not resize the disk", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
				Expect(err).ToNot(HaveOccurred())
				Expect(partitioner.ResizeSinglePartitionCalled).To(BeFalse())
			})

			It("does not partition the disk", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
				Expect(err).ToNot(HaveOccurred())
				Expect(partitioner.PartitionCalled).To(BeFalse())
			})

			It("does not format the disk", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
				Expect(err).ToNot(HaveOccurred())
				Expect(formatter.FormatCalled).To(BeFalse())
			})
		})

		Context("when device real path starts with /dev/mapper/ and is successfully resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/mapper/fake-real-device-path"
			})

			It("partitions that disk device", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
				Expect(err).ToNot(HaveOccurred())

				Expect(partitioner.PartitionDevicePath).To(Equal("/dev/mapper/fake-real-device-path"))
			})

			It("formats the first partition of that device", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

				Expect(err).ToNot(HaveOccurred())
				Expect(formatter.FormatPartitionPaths).To(Equal([]string{"/dev/mapper/fake-real-device-path-part1"}))
			})
		})

		Context("when device real path starts with /dev/nvme and is successfully resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/nvme2n1"
			})

			It("partitions that disk device", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
				Expect(err).ToNot(HaveOccurred())

				Expect(partitioner.PartitionDevicePath).To(Equal("/dev/nvme2n1"))
			})

			It("formats the first partition of that device", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

				Expect(err).ToNot(HaveOccurred())
				Expect(formatter.FormatPartitionPaths).To(Equal([]string{"/dev/nvme2n1p1"}))
			})
		})

		Context("when device path is successfully resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "fake-real-device-path"
			})

			It("partitions the disk", func() {
				err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
				Expect(err).ToNot(HaveOccurred())

				Expect(diskManager.GetPersistentDevicePartitionerCallCount()).To(Equal(1))
				Expect(diskManager.GetPersistentDevicePartitionerArgsForCall(0)).To(Equal(""))

				partitions := []boshdisk.Partition{{Type: boshdisk.PartitionTypeLinux}}
				Expect(partitioner.PartitionDevicePath).To(Equal("fake-real-device-path"))
				Expect(partitioner.PartitionPartitions).To(Equal(partitions))
			})

			Context("when fetching the partitioner fails", func() {
				BeforeEach(func() {
					diskManager.GetPersistentDevicePartitionerReturns(nil, errors.New("boom"))
				})

				It("returns the error", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when a persistent disk partitioner is requested", func() {
				BeforeEach(func() {
					diskSettings.Partitioner = "cool-partitioner"
				})

				It("fetches that partitioner", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
					Expect(err).NotTo(HaveOccurred())
					Expect(diskManager.GetPersistentDevicePartitionerCallCount()).To(Equal(1))
					Expect(diskManager.GetPersistentDevicePartitionerArgsForCall(0)).To(Equal("cool-partitioner"))
				})
			})

			Context("when partition needs resize after IaaS-native disk resize", func() {
				BeforeEach(func() {
					partitioner.SinglePartitionNeedsResizeReturns.NeedResize = true
					partitioner.SinglePartitionNeedsResizeReturns.Err = nil
				})

				It("resizes the single partition", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					Expect(partitioner.ResizeSinglePartitionDevicePath).To(Equal("fake-real-device-path"))
				})

				It("grows the partition filesystem, temporarily mounting it", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					Expect(mounter.MountCallCount()).To(Equal(1))
					arg1, arg2, _ := mounter.MountArgsForCall(0)
					Expect(arg1).To(Equal("fake-real-device-path1"))
					Expect(arg2).To(Equal("/mnt/point"))

					Expect(formatter.GrowFilesystemCalled).To(BeTrue())
					Expect(formatter.GrowFilesystemPartitionPath).To(Equal("fake-real-device-path1"))

					Expect(mounter.UnmountCallCount()).To(Equal(1))
					Expect(mounter.UnmountArgsForCall(0)).To(Equal("fake-real-device-path1"))
				})
			})

			Context("when settings do NOT specify persistentDiskFS", func() {
				It("formats the first partition in ext4 format", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

					Expect(err).ToNot(HaveOccurred())

					Expect(formatter.FormatPartitionPaths).To(Equal([]string{"fake-real-device-path1"}))
					Expect(formatter.FormatFsTypes).To(Equal([]boshdisk.FileSystemType{boshdisk.FileSystemExt4}))
				})
			})

			Context("when settings specify ext4 filesysem", func() {
				BeforeEach(func() {
					diskSettings.FileSystemType = boshdisk.FileSystemExt4
				})

				It("formats with an ext4 filesysem", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

					Expect(err).ToNot(HaveOccurred())
					Expect(formatter.FormatFsTypes).To(Equal([]boshdisk.FileSystemType{boshdisk.FileSystemExt4}))
				})
			})

			Context("when settings specify xfs filesysem", func() {
				BeforeEach(func() {
					diskSettings.FileSystemType = boshdisk.FileSystemXFS
				})

				It("formats with an xfs filesysem", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

					Expect(err).ToNot(HaveOccurred())
					Expect(formatter.FormatFsTypes).To(Equal([]boshdisk.FileSystemType{boshdisk.FileSystemXFS}))
				})
			})

			Context("when settings specify an unsupported filesystem", func() {
				BeforeEach(func() {
					diskSettings.FileSystemType = boshdisk.FileSystemType("blahblah")
				})

				It("it errors", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(`The filesystem type "blahblah" is not supported`))
				})
			})

			Context(`when settings specify a "swap" filesystem`, func() {
				BeforeEach(func() {
					diskSettings.FileSystemType = boshdisk.FileSystemSwap
				})

				It("it errors", func() {
					err := platform.AdjustPersistentDiskPartitioning(diskSettings, mntPoint)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(`The filesystem type "swap" is not supported`))
				})
			})

			Context("when disk could not be formatted", func() {
				BeforeEach(func() {
					formatter.FormatError = errors.New("oh noes")
				})

				It("returns an error", func() {
					err := platform.AdjustPersistentDiskPartitioning(
						boshsettings.DiskSettings{Path: "fake-volume-id", FileSystemType: boshdisk.FileSystemXFS},
						"/mnt/point",
					)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Formatting partition with xfs: oh noes"))
				})
			})
		})
	})

	Describe("MountPersistentDisk", func() {
		var (
			diskSettings boshsettings.DiskSettings
			mntPoint     string
		)

		BeforeEach(func() {
			diskSettings = boshsettings.DiskSettings{ID: "fake-unique-id", Path: "fake-volume-id", MountOptions: []string{"mntOpt1", "mntOpt2"}}
			mntPoint = "/mnt/point"
		})

		Context("when device real path starts with /dev/mapper/ and is successfully resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/mapper/fake-real-device-path"
			})

			Context("when store directory is already mounted", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", true, nil)
				})

				Context("when mounting the same device", func() {
					BeforeEach(func() {
						mounter.IsMountPointReturns("/dev/mapper/fake-real-device-path-part1", true, nil)
					})

					It("skips mounting", func() {
						err := platform.MountPersistentDisk(diskSettings, mntPoint)
						Expect(err).ToNot(HaveOccurred())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})

				Context("when mounting a different device", func() {
					BeforeEach(func() {
						mounter.IsMountPointReturns("/dev/mapper/another-device", true, nil)
					})

					It("mounts the store migration directory", func() {
						err := platform.MountPersistentDisk(diskSettings, mntPoint)
						Expect(err).ToNot(HaveOccurred())
						Expect(fs.GetFileTestStat("/fake-dir/store_migration_target").FileType).To(Equal(fakesys.FakeFileTypeDir))

						Expect(mounter.MountCallCount()).To(Equal(1))
						partition, mntPt, options := mounter.MountArgsForCall(0)
						Expect(partition).To(Equal("/dev/mapper/fake-real-device-path-part1"))
						Expect(mntPt).To(Equal("/fake-dir/store_migration_target"))
						Expect(options).To(Equal([]string{"mntOpt1", "mntOpt2"}))
					})
				})
			})

			Context("when failing to determine if store directory is mounted", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", false, errors.New("fake-is-mount-point-err"))
				})

				It("returns an error", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-mount-point-err"))
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})

			Context("when UsePreformattedPersistentDisk set to false", func() {
				It("creates the mount directory with the correct permissions", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					mountPoint := fs.GetFileTestStat("/mnt/point")
					Expect(mountPoint.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(mountPoint.FileMode).To(Equal(os.FileMode(0700)))
				})

				It("returns error when creating mount directory fails", func() {
					fs.MkdirAllError = errors.New("fake-mkdir-all-err")

					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mkdir-all-err"))
				})

				It("mounts the disk", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					Expect(mounter.MountCallCount()).To(Equal(1))
					partition, mntPt, options := mounter.MountArgsForCall(0)
					Expect(partition).To(Equal("/dev/mapper/fake-real-device-path-part1"))
					Expect(mntPt).To(Equal("/mnt/point"))
					Expect(options).To(Equal([]string{"mntOpt1", "mntOpt2"}))
				})

				It("generates the managed disk settings file", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					var contents string
					managedSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")

					contents, err = platform.GetFs().ReadFileString(managedSettingsPath)
					Expect(err).ToNot(HaveOccurred())
					Expect(contents).To(Equal("fake-unique-id"))
				})
			})
		})

		Context("when device real path starts with /dev/nvme and is successfully resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/nvme2n1"
			})

			Context("when store directory is already mounted", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", true, nil)
				})

				Context("when mounting the same device", func() {
					BeforeEach(func() {
						mounter.IsMountPointReturns("/dev/nvme2n1p1", true, nil)
					})

					It("skips mounting", func() {
						err := platform.MountPersistentDisk(diskSettings, mntPoint)
						Expect(err).ToNot(HaveOccurred())
						Expect(mounter.MountCallCount()).To(Equal(0))
					})
				})

				Context("when mounting a different device", func() {
					BeforeEach(func() {
						mounter.IsMountPointReturns("/dev/nvme3n1p1", true, nil)
					})

					It("mounts the store migration directory", func() {
						err := platform.MountPersistentDisk(diskSettings, mntPoint)
						Expect(err).ToNot(HaveOccurred())
						Expect(fs.GetFileTestStat("/fake-dir/store_migration_target").FileType).To(Equal(fakesys.FakeFileTypeDir))

						Expect(mounter.MountCallCount()).To(Equal(1))
						partition, mntPt, options := mounter.MountArgsForCall(0)
						Expect(partition).To(Equal("/dev/nvme2n1p1"))
						Expect(mntPt).To(Equal("/fake-dir/store_migration_target"))
						Expect(options).To(Equal([]string{"mntOpt1", "mntOpt2"}))
					})
				})
			})

			Context("when failing to determine if store directory is mounted", func() {
				BeforeEach(func() {
					mounter.IsMountPointReturns("", false, errors.New("fake-is-mount-point-err"))
				})

				It("returns an error", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-is-mount-point-err"))
					Expect(mounter.MountCallCount()).To(Equal(0))
				})
			})

			Context("when UsePreformattedPersistentDisk set to false", func() {
				It("creates the mount directory with the correct permissions", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					mountPoint := fs.GetFileTestStat("/mnt/point")
					Expect(mountPoint.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(mountPoint.FileMode).To(Equal(os.FileMode(0700)))
				})

				It("returns error when creating mount directory fails", func() {
					fs.MkdirAllError = errors.New("fake-mkdir-all-err")

					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mkdir-all-err"))
				})

				It("mounts the disk", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					Expect(mounter.MountCallCount()).To(Equal(1))
					partition, mntPt, options := mounter.MountArgsForCall(0)
					Expect(partition).To(Equal("/dev/nvme2n1p1"))
					Expect(mntPt).To(Equal("/mnt/point"))
					Expect(options).To(Equal([]string{"mntOpt1", "mntOpt2"}))
				})

				It("generates the managed disk settings file", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					var contents string
					managedSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")

					contents, err = platform.GetFs().ReadFileString(managedSettingsPath)
					Expect(err).ToNot(HaveOccurred())
					Expect(contents).To(Equal("fake-unique-id"))
				})
			})
		})

		Context("when device path is successfully resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "fake-real-device-path"
			})

			Context("when UsePreformattedPersistentDisk set to false", func() {
				It("creates the mount directory with the correct permissions", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					mountPoint := fs.GetFileTestStat("/mnt/point")
					Expect(mountPoint.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(mountPoint.FileMode).To(Equal(os.FileMode(0700)))
				})

				It("returns error when creating mount directory fails", func() {
					fs.MkdirAllError = errors.New("fake-mkdir-all-err")

					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mkdir-all-err"))
				})

				It("returns an error when updating managed_disk_settings.json fails", func() {
					fs.WriteFileError = errors.New("fake error")

					err := platform.MountPersistentDisk(
						boshsettings.DiskSettings{Path: "fake-volume-id", FileSystemType: boshdisk.FileSystemXFS},
						"/mnt/point",
					)

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Writing managed_disk_settings.json: fake error"))
				})

				It("mounts the disk", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					Expect(mounter.MountCallCount()).To(Equal(1))
					partition, mntPt, options := mounter.MountArgsForCall(0)
					Expect(partition).To(Equal("fake-real-device-path1"))
					Expect(mntPt).To(Equal("/mnt/point"))
					Expect(options).To(Equal([]string{"mntOpt1", "mntOpt2"}))
				})
			})

			Context("when UsePreformattedPersistentDisk set to true", func() {
				BeforeEach(func() {
					options.UsePreformattedPersistentDisk = true
				})

				It("creates the mount directory with the correct permissions", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					mountPoint := fs.GetFileTestStat("/mnt/point")
					Expect(mountPoint.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(mountPoint.FileMode).To(Equal(os.FileMode(0700)))
				})

				It("returns error when creating mount directory fails", func() {
					fs.MkdirAllError = errors.New("fake-mkdir-all-err")

					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mkdir-all-err"))
				})

				It("mounts volume at mount point", func() {
					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).ToNot(HaveOccurred())

					Expect(mounter.MountCallCount()).To(Equal(1))
					partition, mntPt, options := mounter.MountArgsForCall(0)
					Expect(partition).To(Equal("fake-real-device-path"))
					Expect(mntPt).To(Equal("/mnt/point"))
					Expect(options).To(Equal([]string{"mntOpt1", "mntOpt2"}))
				})

				It("returns error when mounting fails", func() {
					mounter.MountReturns(errors.New("fake-mount-err"))

					err := platform.MountPersistentDisk(diskSettings, mntPoint)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-mount-err"))
				})
			})
		})

		Context("when device path is not successfully resolved", func() {
			It("return an error", func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-get-real-device-path-err")

				err := platform.MountPersistentDisk(diskSettings, mntPoint)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-real-device-path-err"))
			})
		})
	})

	Describe("UnmountPersistentDisk", func() {
		ItUnmountsPersistentDisk := func(expectedUnmountMountPoint string) {
			It("returs true without an error if unmounting succeeded", func() {
				mounter.UnmountReturns(true, nil)

				didUnmount, err := platform.UnmountPersistentDisk(boshsettings.DiskSettings{})
				Expect(err).NotTo(HaveOccurred())
				Expect(didUnmount).To(BeTrue())
				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(mounter.UnmountArgsForCall(0)).To(Equal(expectedUnmountMountPoint))
			})

			It("returs false without an error if was already unmounted", func() {
				mounter.UnmountReturns(false, nil)

				didUnmount, err := platform.UnmountPersistentDisk(boshsettings.DiskSettings{})
				Expect(err).NotTo(HaveOccurred())
				Expect(didUnmount).To(BeFalse())
				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(mounter.UnmountArgsForCall(0)).To(Equal(expectedUnmountMountPoint))
			})

			It("returns error if unmounting fails", func() {
				mounter.UnmountReturns(false, errors.New("fake-unmount-err"))

				didUnmount, err := platform.UnmountPersistentDisk(boshsettings.DiskSettings{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-unmount-err"))
				Expect(didUnmount).To(BeFalse())
				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(mounter.UnmountArgsForCall(0)).To(Equal(expectedUnmountMountPoint))
			})
		}

		Context("when device real path contains /dev/mapper/ and can be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/mapper/fake-real-device-path"
			})

			Context("UsePreformattedPersistentDisk is set to false", func() {
				ItUnmountsPersistentDisk("/dev/mapper/fake-real-device-path-part1") // note partition '-part1'
			})
		})

		Context("when device real path contains /dev/nvme and can be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/nvme2n1"
			})

			Context("UsePreformattedPersistentDisk is set to false", func() {
				ItUnmountsPersistentDisk("/dev/nvme2n1p1") // note partition 'p1'
			})
		})

		Context("when device path can be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "fake-real-device-path"
			})

			Context("UsePreformattedPersistentDisk is set to false", func() {
				ItUnmountsPersistentDisk("fake-real-device-path1") // note partition '1'
			})

			Context("UsePreformattedPersistentDisk is set to true", func() {
				BeforeEach(func() {
					options.UsePreformattedPersistentDisk = true
				})

				ItUnmountsPersistentDisk("fake-real-device-path") // note no '1'; no partitions
			})
		})

		Context("when device path cannot be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-get-real-device-path-err")
				devicePathResolver.GetRealDevicePathTimedOut = false
			})

			It("returns error", func() {
				isMounted, err := platform.UnmountPersistentDisk(boshsettings.DiskSettings{Path: "fake-device-path"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-real-device-path-err"))
				Expect(isMounted).To(BeFalse())
			})
		})

		Context("when device path cannot be resolved due to timeout", func() {
			BeforeEach(func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-get-real-device-path-err")
				devicePathResolver.GetRealDevicePathTimedOut = true
			})

			It("does not return error", func() {
				isMounted, err := platform.UnmountPersistentDisk(boshsettings.DiskSettings{Path: "fake-device-path"})
				Expect(err).NotTo(HaveOccurred())
				Expect(isMounted).To(BeFalse())
			})
		})
	})

	Describe("AssociateDisk", func() {
		var (
			diskName     string
			diskSettings boshsettings.DiskSettings
		)

		BeforeEach(func() {
			diskName = "cool_disk"
			diskSettings = boshsettings.DiskSettings{Path: "/dev/path/to/cool_disk"}
			devicePathResolver.RealDevicePath = "/dev/path/to/cool_disk"
		})

		It("Creates a symlink a disk", func() {
			err := platform.AssociateDisk(diskName, diskSettings)
			Expect(err).ToNot(HaveOccurred())

			path, err := fs.Readlink("/fake-dir/instance/disks/cool_disk")
			Expect(err).ToNot(HaveOccurred())
			Expect(path).To(Equal("/dev/path/to/cool_disk"))
		})

		Context("when discovering real device path returns an error", func() {
			BeforeEach(func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("barf")
			})

			It("returns an error", func() {
				err := platform.AssociateDisk(diskName, diskSettings)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when symlinking returns an error", func() {
			BeforeEach(func() {
				fs.SymlinkError = &os.LinkError{
					Op:  "Symlink",
					Old: "/dev/path/to/cool_disk",
					New: "/fake-dir/instance/disks/cool_disk",
					Err: errors.New("some linking error"),
				}
			})

			It("returns an error", func() {
				err := platform.AssociateDisk(diskName, diskSettings)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetFileContentsFromCDROM", func() {
		It("delegates to cdutil", func() {
			cdutil.GetFilesContentsContents = [][]byte{[]byte("fake-contents")}
			filename := "fake-env"
			contents, err := platform.GetFileContentsFromCDROM(filename)
			Expect(err).NotTo(HaveOccurred())
			Expect(cdutil.GetFilesContentsFileNames[0]).To(Equal(filename))
			Expect(contents).To(Equal([]byte("fake-contents")))
		})
	})

	Describe("GetFilesContentsFromDisk", func() {
		It("delegates to diskutil", func() {
			diskUtil.GetFilesContentsContents = [][]byte{
				[]byte("fake-contents-1"),
				[]byte("fake-contents-2"),
			}
			contents, err := platform.GetFilesContentsFromDisk(
				"fake-disk-path",
				[]string{"fake-file-path-1", "fake-file-path-2"},
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(diskUtil.GetFilesContentsDiskPath).To(Equal("fake-disk-path"))
			Expect(diskUtil.GetFilesContentsFileNames).To(Equal(
				[]string{"fake-file-path-1", "fake-file-path-2"},
			))
			Expect(contents).To(Equal([][]byte{
				[]byte("fake-contents-1"),
				[]byte("fake-contents-2"),
			}))
		})
	})

	Describe("GetEphemeralDiskPath", func() {
		Context("when real device path was resolved without an error", func() {
			It("returns real device path and true", func() {
				devicePathResolver.RealDevicePath = "fake-real-device-path"
				realPath, _ := platform.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: "fake-device-path"}) //nolint:errcheck
				Expect(realPath).To(Equal("fake-real-device-path"))
			})
		})

		Context("when real device path was not resolved without an error", func() {
			It("returns real device path and true", func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-get-real-device-path-err")
				realPath, _ := platform.GetEphemeralDiskPath(boshsettings.DiskSettings{Path: "fake-device-path"}) //nolint:errcheck
				Expect(realPath).To(Equal(""))
			})
		})
	})

	Describe("MigratePersistentDisk", func() {
		It("migrate persistent disk", func() {
			err := platform.MigratePersistentDisk("/from/path", "/to/path")
			Expect(err).ToNot(HaveOccurred())

			Expect(mounter.RemountAsReadonlyCallCount()).To(Equal(1))
			Expect(mounter.RemountAsReadonlyArgsForCall(0)).To(Equal("/from/path"))

			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"sh", "-c", "(tar -C /from/path --xattrs --xattrs-include=*.* --sparse -cf - .) | (tar -C /to/path --xattrs --xattrs-include=*.* -xpf -)"}))

			Expect(mounter.UnmountCallCount()).To(Equal(1))
			Expect(mounter.UnmountArgsForCall(0)).To(Equal("/from/path"))

			Expect(mounter.RemountCallCount()).To(Equal(1))
			fromPath, toPath, options := mounter.RemountArgsForCall(0)
			Expect(fromPath).To(Equal("/to/path"))
			Expect(toPath).To(Equal("/from/path"))
			Expect(options).To(BeEmpty())
		})

		Context("when device path resolution type is iscsi", func() {
			BeforeEach(func() {
				mountsSearcher.SearchMountsMounts = []boshdisk.Mount{
					{PartitionPath: "/dev/mapper/from-device-path-part1", MountPoint: "/from/path"},
					{PartitionPath: "/dev/mapper/to-device-path-part1", MountPoint: "/to/path"},
				}
				cmdRunner.AddCmdResult("multipath -ll", fakesys.FakeCmdResult{
					Stdout: `to-device-path  dm-2 NETAPP  ,LUN C-Mode
from-device-path  dm-0 NETAPP  ,LUN C-Mode
`,
					Stderr: "",
					Error:  nil,
				})
			})

			It("migrate persistent disk", func() {
				var platformWithISCSIType Platform

				options.DevicePathResolutionType = "iscsi"
				platformWithISCSIType = NewLinuxPlatform(
					fs,
					cmdRunner,
					collector,
					compressor,
					copier,
					dirProvider,
					vitalsService,
					cdutil,
					diskManager,
					netManager,
					certManager,
					monitRetryStrategy,
					devicePathResolver,
					state,
					options,
					logger,
					fakeDefaultNetworkResolver,
					fakeUUIDGenerator,
					fakeAuditLogger,
					fakeLogsTarProvider,
					serviceManager,
				)

				err := platformWithISCSIType.MigratePersistentDisk("/from/path", "/to/path")
				Expect(err).ToNot(HaveOccurred())

				Expect(mounter.RemountAsReadonlyCallCount()).To(Equal(1))
				Expect(mounter.RemountAsReadonlyArgsForCall(0)).To(Equal("/from/path"))

				Expect(len(cmdRunner.RunCommands)).To(Equal(3))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"sh", "-c", "(tar -C /from/path --xattrs --xattrs-include=*.* --sparse -cf - .) | (tar -C /to/path --xattrs --xattrs-include=*.* -xpf -)"}))

				Expect(mounter.UnmountCallCount()).To(Equal(1))
				Expect(mounter.UnmountArgsForCall(0)).To(Equal("/from/path"))

				Expect(mounter.RemountCallCount()).To(Equal(1))
				fromPath, toPath, options := mounter.RemountArgsForCall(0)
				Expect(fromPath).To(Equal("/to/path"))
				Expect(toPath).To(Equal("/from/path"))
				Expect(options).To(BeEmpty())

				Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"multipath", "-ll"}))
				Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"multipath", "-f", "from-device-path"}))
			})
		})
	})

	Describe("IsPersistentDiskMounted", func() {
		act := func() (bool, error) {
			return platform.IsPersistentDiskMounted(boshsettings.DiskSettings{Path: "fake-device-path"})
		}

		Context("when device real path contains /dev/mapper/ and can be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/mapper/fake-real-device-path"
			})

			ItChecksPersistentDiskMountPoint := func(expectedCheckedMountPoint string) {
				Context("when checking persistent disk mount point succeeds", func() {
					It("returns true if mount point exists", func() {
						mounter.IsMountedReturns(true, nil)

						isMounted, err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(isMounted).To(BeTrue())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})

					It("returns false if mount point does not exist", func() {
						mounter.IsMountedReturns(false, nil)

						isMounted, err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(isMounted).To(BeFalse())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})
				})

				Context("checking persistent disk mount points fails", func() {
					It("returns error", func() {
						mounter.IsMountedReturns(false, errors.New("fake-is-mounted-err"))

						isMounted, err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-is-mounted-err"))
						Expect(isMounted).To(BeFalse())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})
				})
			}

			Context("UsePreformattedPersistentDisk is set to false", func() {
				ItChecksPersistentDiskMountPoint("/dev/mapper/fake-real-device-path-part1") // note partition '-part1'
			})
		})

		Context("when device real path starts with /dev/nvme and can be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "/dev/nvme2n1"
			})

			ItChecksPersistentDiskMountPoint := func(expectedCheckedMountPoint string) {
				Context("when checking persistent disk mount point succeeds", func() {
					It("returns true if mount point exists", func() {
						mounter.IsMountedReturns(true, nil)

						isMounted, err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(isMounted).To(BeTrue())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})

					It("returns false if mount point does not exist", func() {
						mounter.IsMountedReturns(false, nil)

						isMounted, err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(isMounted).To(BeFalse())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})
				})

				Context("checking persistent disk mount points fails", func() {
					It("returns error", func() {
						mounter.IsMountedReturns(false, errors.New("fake-is-mounted-err"))

						isMounted, err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-is-mounted-err"))
						Expect(isMounted).To(BeFalse())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})
				})
			}

			Context("UsePreformattedPersistentDisk is set to false", func() {
				ItChecksPersistentDiskMountPoint("/dev/nvme2n1p1") // note partition 'p1'
			})
		})

		Context("when device path can be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.RealDevicePath = "fake-real-device-path"
			})

			ItChecksPersistentDiskMountPoint := func(expectedCheckedMountPoint string) {
				Context("when checking persistent disk mount point succeeds", func() {
					It("returns true if mount point exists", func() {
						mounter.IsMountedReturns(true, nil)

						isMounted, err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(isMounted).To(BeTrue())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})

					It("returns false if mount point does not exist", func() {
						mounter.IsMountedReturns(false, nil)

						isMounted, err := act()
						Expect(err).NotTo(HaveOccurred())
						Expect(isMounted).To(BeFalse())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})
				})

				Context("checking persistent disk mount points fails", func() {
					It("returns error", func() {
						mounter.IsMountedReturns(false, errors.New("fake-is-mounted-err"))

						isMounted, err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-is-mounted-err"))
						Expect(isMounted).To(BeFalse())
						Expect(mounter.IsMountedArgsForCall(0)).To(Equal(expectedCheckedMountPoint))
					})
				})
			}

			Context("UsePreformattedPersistentDisk is set to false", func() {
				ItChecksPersistentDiskMountPoint("fake-real-device-path1") // note partition '1'
			})

			Context("UsePreformattedPersistentDisk is set to true", func() {
				BeforeEach(func() {
					options.UsePreformattedPersistentDisk = true
				})

				ItChecksPersistentDiskMountPoint("fake-real-device-path") // note no '1'; no partitions
			})
		})

		Context("when device path cannot be resolved", func() {
			BeforeEach(func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-get-real-device-path-err")
				devicePathResolver.GetRealDevicePathTimedOut = false
			})

			It("returns error", func() {
				isMounted, err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-real-device-path-err"))
				Expect(isMounted).To(BeFalse())
			})
		})

		Context("when device path cannot be resolved due to timeout", func() {
			BeforeEach(func() {
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-get-real-device-path-err")
				devicePathResolver.GetRealDevicePathTimedOut = true
			})

			It("does not return error", func() {
				isMounted, err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(isMounted).To(BeFalse())
			})
		})
	})

	Describe("IsPersistentDiskMountable", func() {
		BeforeEach(func() {
			devicePathResolver.RealDevicePath = "/fake/device"
		})

		Context("when the specified drive does not exist", func() {
			It("returns error", func() {
				devicePathResolver.GetRealDevicePathTimedOut = true
				devicePathResolver.GetRealDevicePathErr = errors.New("fake-timeout-error")
				diskSettings := boshsettings.DiskSettings{
					Path: "/fake/device",
				}

				isMounted, err := platform.IsPersistentDiskMountable(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(isMounted).To(Equal(false))
			})
		})

		Context("when there is no partition on drive", func() {
			It("returns false", func() {
				result := fakesys.FakeCmdResult{
					Error:      nil,
					ExitStatus: 0,
					Stderr: `
dfdisk: ERROR: sector 0 does not have an msdos signature
/fake/device: unrecognized partition table type
No partitions found
`,
					Stdout: "",
				}

				cmdRunner.AddCmdResult("sfdisk -d /fake/device", result)

				diskSettings := boshsettings.DiskSettings{
					Path: "/fake/device",
				}

				isMounted, err := platform.IsPersistentDiskMountable(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(isMounted).To(Equal(false))
			})
		})

		Context("when drive is partitioned", func() {
			It("returns true", func() {
				result := fakesys.FakeCmdResult{
					Error:      nil,
					ExitStatus: 0,
					Stderr:     "",
					Stdout: `# partition table of /fake/device
unit: sectors

/fake/device1 : start=       63, size=  5997984, Id=83
/fake/device2 : start=  5998592, size= 32691088, Id=83
/fake/device3 : start= 38690816, size=195750832, Id=83
/fake/device4 : start=        0, size=        0, Id= 0
`,
				}

				cmdRunner.AddCmdResult("sfdisk -d /fake/device", result)

				diskSettings := boshsettings.DiskSettings{
					Path: "/fake/device",
				}

				isMounted, err := platform.IsPersistentDiskMountable(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(isMounted).To(Equal(true))
			})
		})
	})

	Describe("StartMonit", func() {
		It("sets up the service", func() {
			err := platform.StartMonit()
			Expect(err).NotTo(HaveOccurred())

			Expect(serviceManager.SetupCallCount()).To(Equal(1))
			Expect(serviceManager.SetupArgsForCall(0)).To(Equal("monit"))
		})

		It("retries to start monit", func() {
			err := platform.StartMonit()
			Expect(err).NotTo(HaveOccurred())
			Expect(monitRetryStrategy.TryCalled).To(BeTrue())
		})

		It("returns error if retrying to start monit fails", func() {
			monitRetryStrategy.TryErr = errors.New("fake-retry-monit-error")

			err := platform.StartMonit()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-retry-monit-error"))
		})
	})

	Describe("SetupMonitUser", func() {
		It("setup monit user", func() {
			err := platform.SetupMonitUser()
			Expect(err).NotTo(HaveOccurred())

			monitUserFileStats := fs.GetFileTestStat("/fake-dir/monit/monit.user")
			Expect(monitUserFileStats).ToNot(BeNil())
			Expect(monitUserFileStats.StringContents()).To(Equal("vcap:random-password"))
		})
	})

	Describe("GetMonitCredentials", func() {
		It("get monit credentials reads monit file from disk", func() {
			err := fs.WriteFileString("/fake-dir/monit/monit.user", "fake-user:fake-random-password")
			Expect(err).NotTo(HaveOccurred())

			username, password, err := platform.GetMonitCredentials()
			Expect(err).NotTo(HaveOccurred())

			Expect(username).To(Equal("fake-user"))
			Expect(password).To(Equal("fake-random-password"))
		})

		It("get monit credentials errs when invalid file format", func() {
			err := fs.WriteFileString("/fake-dir/monit/monit.user", "fake-user")
			Expect(err).NotTo(HaveOccurred())

			_, _, err = platform.GetMonitCredentials()
			Expect(err).To(HaveOccurred())
		})

		It("get monit credentials leaves colons in password intact", func() {
			err := fs.WriteFileString("/fake-dir/monit/monit.user", "fake-user:fake:random:password")
			Expect(err).NotTo(HaveOccurred())

			username, password, err := platform.GetMonitCredentials()
			Expect(err).NotTo(HaveOccurred())

			Expect(username).To(Equal("fake-user"))
			Expect(password).To(Equal("fake:random:password"))
		})
	})

	Describe("PrepareForNetworkingChange", func() {
		It("removes the network persistent rules file", func() {
			err := fs.WriteFile("/etc/udev/rules.d/70-persistent-net.rules", []byte{})
			Expect(err).NotTo(HaveOccurred())

			err = platform.PrepareForNetworkingChange()
			Expect(err).NotTo(HaveOccurred())

			Expect(fs.FileExists("/etc/udev/rules.d/70-persistent-net.rules")).To(BeFalse())
		})

		It("returns error if removing persistent rules file fails", func() {
			fs.RemoveAllStub = func(_ string) error {
				return errors.New("fake-remove-all-error")
			}

			err := platform.PrepareForNetworkingChange()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-remove-all-error"))
		})
	})

	Describe("SetupIPv6", func() {
		It("delegates to the NetManager", func() {
			netManager.SetupIPv6Err = errors.New("fake-err")

			err := platform.SetupIPv6(boshsettings.IPv6{Enable: true})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-err"))

			Expect(netManager.SetupIPv6Config).To(Equal(boshsettings.IPv6{Enable: true}))
			Expect(netManager.SetupIPv6StopCh).To(BeNil())
		})
	})

	Describe("SetupNetworking", func() {
		It("delegates to the NetManager", func() {
			networks := boshsettings.Networks{}

			err := platform.SetupNetworking(networks, "")
			Expect(err).ToNot(HaveOccurred())

			Expect(netManager.SetupNetworkingNetworks).To(Equal(networks))
		})
	})

	Describe("GetConfiguredNetworkInterfaces", func() {
		It("delegates to the NetManager", func() {
			netmanagerInterfaces := []string{"fake-eth0", "fake-eth1"}
			netManager.GetConfiguredNetworkInterfacesInterfaces = netmanagerInterfaces

			interfaces, err := platform.GetConfiguredNetworkInterfaces()
			Expect(err).ToNot(HaveOccurred())
			Expect(interfaces).To(Equal(netmanagerInterfaces))
		})
	})

	Describe("GetDefaultNetwork", func() {
		for _, ipProtocol := range []boship.IPProtocol{boship.IPv4, boship.IPv6} {
			title := fmt.Sprintf("delegates to the defaultNetworkResolver with input param %s", ipProtocol)
			It(title, func() {
				defaultNetwork := boshsettings.Network{}
				fakeDefaultNetworkResolver.GetDefaultNetworkNetwork = defaultNetwork
				network, err := platform.GetDefaultNetwork(ipProtocol)
				Expect(err).ToNot(HaveOccurred())
				Expect(network).To(Equal(defaultNetwork))
				Expect(fakeDefaultNetworkResolver.GetDefaultNetworkCalledWith).To(Equal(ipProtocol))
			})
		}
	})

	Describe("GetHostPublicKey", func() {
		It("gets host public key if file exists", func() {
			err := fs.WriteFileString("/etc/ssh/ssh_host_rsa_key.pub", "public-key")
			Expect(err).NotTo(HaveOccurred())

			hostPublicKey, err := platform.GetHostPublicKey()
			Expect(err).ToNot(HaveOccurred())
			Expect(hostPublicKey).To(Equal("public-key"))
		})

		It("throws error if file does not exist", func() {
			hostPublicKey, err := platform.GetHostPublicKey()
			Expect(err).To(HaveOccurred())
			Expect(hostPublicKey).To(Equal(""))
		})
	})

	Describe("DeleteARPEntryWithIP", func() {
		It("cleans the arp entry for the given ip", func() {
			err := platform.DeleteARPEntryWithIP("1.2.3.4")
			deleteArpEntry := []string{"ip", "neigh", "flush", "to", "1.2.3.4"}
			Expect(cmdRunner.RunCommands[0]).To(Equal(deleteArpEntry))
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails when failing to clear arp cache", func() {
			result := fakesys.FakeCmdResult{
				Error:      errors.New("failure"),
				ExitStatus: 1,
				Stderr:     "",
				Stdout:     "",
			}
			cmdRunner.AddCmdResult("ip neigh flush to 1.2.3.4", result)

			err := platform.DeleteARPEntryWithIP("1.2.3.4")

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Shutdown", func() {
		It("shuts the system down", func() {
			err := platform.Shutdown()
			Expect(err).ToNot(HaveOccurred())

			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"shutdown", "-P", "0"}))
		})

		It("fails if shutdown command failed", func() {
			result := fakesys.FakeCmdResult{
				Error: errors.New("shutdown: Unable to shutdown system"),
			}
			cmdRunner.AddCmdResult("shutdown -P 0", result)

			err := platform.Shutdown()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("RemoveDevTools", func() {
		It("removes listed packages", func() {
			devToolsListPath := path.Join(dirProvider.EtcDir(), "dev_tools_file_list")
			err := fs.WriteFileString(devToolsListPath, "dummy-compiler")
			Expect(err).NotTo(HaveOccurred())
			err = platform.RemoveDevTools(devToolsListPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(cmdRunner.RunCommands)).To(Equal(1))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"rm", "-rf", "dummy-compiler"}))
		})
	})

	Describe("RemoveStaticLibraries", func() {
		It("removes listed static libraries", func() {
			staticLibrariesListPath := path.Join(dirProvider.EtcDir(), "static_libraries_list")
			err := fs.WriteFileString(staticLibrariesListPath, "static.a\nlibrary.a")
			Expect(err).NotTo(HaveOccurred())
			err = platform.RemoveStaticLibraries(staticLibrariesListPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(cmdRunner.RunCommands).To(HaveLen(2))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"rm", "-rf", "static.a"}))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"rm", "-rf", "library.a"}))
		})

		Context("when there is an error reading the static libraries list file", func() {
			It("should return an error", func() {
				err := platform.RemoveStaticLibraries("non-existent-path")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unable to read static libraries list file"))
			})
		})

		Context("when there is an error removing a static library", func() {
			It("should return an error", func() {
				cmdRunner.AddCmdResult("rm -rf library.a", fakesys.FakeCmdResult{Error: errors.New("oh noes")})
				staticLibrariesListPath := path.Join(dirProvider.EtcDir(), "static_libraries_list")
				err := fs.WriteFileString(staticLibrariesListPath, "static.a\nlibrary.a")
				Expect(err).NotTo(HaveOccurred())
				err = platform.RemoveStaticLibraries(staticLibrariesListPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("oh noes"))
				Expect(cmdRunner.RunCommands).To(HaveLen(2))
			})
		})
	})

	Describe("SaveDNSRecords", func() {
		var (
			dnsRecords boshsettings.DNSRecords

			defaultEtcHosts string
		)

		BeforeEach(func() {
			dnsRecords = boshsettings.DNSRecords{
				Records: [][2]string{
					{"fake-ip0", "fake-name0"},
					{"fake-ip1", "fake-name1"},
				},
			}

			defaultEtcHosts = strings.ReplaceAll(EtcHostsTemplate, "{{ . }}", "fake-hostname")
		})

		It("fails generating a UUID", func() {
			fakeUUIDGenerator.GenerateError = errors.New("fake-error")

			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Generating UUID"))
		})

		It("fails to create intermediary /etc/hosts-<uuid> file", func() {
			fs.WriteFileErrors["/etc/hosts-fake-uuid-0"] = errors.New("fake-error")

			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("Writing to /etc/hosts-fake-uuid-0"))
		})

		It("fails to renames intermediary /etc/hosts-<uuid> file to /etc/hosts", func() {
			fs.RenameError = errors.New("fake-error")

			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("Renaming /etc/hosts-fake-uuid-0 to /etc/hosts"))
		})

		It("renames intermediary /etc/hosts-<uuid> atomically to /etc/hosts", func() {
			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).ToNot(HaveOccurred())

			Expect(fs.RenameError).ToNot(HaveOccurred())

			Expect(len(fs.RenameOldPaths)).To(Equal(1))
			Expect(fs.RenameOldPaths).To(ContainElement("/etc/hosts-fake-uuid-0"))

			Expect(len(fs.RenameNewPaths)).To(Equal(1))
			Expect(fs.RenameNewPaths).To(ContainElement("/etc/hosts"))
		})

		It("preserves the default DNS records in '/etc/hosts'", func() {
			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).ToNot(HaveOccurred())

			hostsFileContents, err := fs.ReadFile("/etc/hosts")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(hostsFileContents)).To(ContainSubstring(defaultEtcHosts))
		})

		It("writes the new DNS records in '/etc/hosts'", func() {
			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).ToNot(HaveOccurred())

			hostsFileContents, err := fs.ReadFile("/etc/hosts")
			Expect(err).ToNot(HaveOccurred())

			Expect(hostsFileContents).Should(MatchRegexp("fake-ip0\\s+fake-name0\\n"))
			Expect(hostsFileContents).Should(MatchRegexp("fake-ip1\\s+fake-name1\\n"))
		})

		It("writes DNS records quietly when asked", func() {
			err := platform.SaveDNSRecords(dnsRecords, "fake-hostname")
			Expect(err).ToNot(HaveOccurred())
			Expect(fs.WriteFileCallCount).To(Equal(0))
			Expect(fs.WriteFileQuietlyCallCount).To(Equal(1))
		})
	})

	Describe("SetupDNSRecordFile", func() {

		It("creates a DNS record file with specific permissions", func() {
			recordsJSONFile, err := platform.GetFs().TempFile("records_json")
			Expect(err).ToNot(HaveOccurred())

			err = platform.SetupRecordsJSONPermission(recordsJSONFile.Name())
			Expect(err).NotTo(HaveOccurred())

			basePathStat := fs.GetFileTestStat(recordsJSONFile.Name())

			Expect(basePathStat).ToNot(BeNil())
			Expect(basePathStat.FileType).To(Equal(fakesys.FakeFileTypeFile))
			Expect(basePathStat.FileMode).To(Equal(os.FileMode(0640)))
			Expect(basePathStat.Username).To(Equal("root"))
			Expect(basePathStat.Groupname).To(Equal("vcap"))
		})

		Context("when chmod fails", func() {
			BeforeEach(func() {
				fs.ChmodErr = errors.New("some chmod error")
			})

			It("should return error", func() {
				recordsJSONFile, err := platform.GetFs().TempFile("records_json")
				Expect(err).ToNot(HaveOccurred())

				err = platform.SetupRecordsJSONPermission(recordsJSONFile.Name())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Chmoding records JSON file: some chmod error"))
			})
		})

		Context("when chown fails", func() {
			BeforeEach(func() {
				fs.ChownErr = errors.New("some chown error")
			})

			It("should return error", func() {
				recordsJSONFile, err := platform.GetFs().TempFile("records_json")
				Expect(err).ToNot(HaveOccurred())

				err = platform.SetupRecordsJSONPermission(recordsJSONFile.Name())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Chowning records JSON file: some chown error"))
			})
		})
	})

	Describe("GetAgentSettingsPath", func() {
		It("returns a path in the sensitive settings directory if tmpfs is enabled", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "settings", "settings.json")

			path := platform.GetAgentSettingsPath(true)
			Expect(path).To(Equal(expectedPath))
		})

		It("returns a path in the default directory if tmpfs is disabled", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "settings.json")

			path := platform.GetAgentSettingsPath(false)
			Expect(path).To(Equal(expectedPath))
		})
	})

	Describe("GetPersistentDiskSettingsPath", func() {
		It("returns a path in the sensitive settings directory if tmpfs is enabled", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "settings", "persistent_disk_hints.json")

			path := platform.GetPersistentDiskSettingsPath(true)
			Expect(path).To(Equal(expectedPath))
		})

		It("returns a path in the default directory if tmpfs is disabled", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "persistent_disk_hints.json")

			path := platform.GetPersistentDiskSettingsPath(false)
			Expect(path).To(Equal(expectedPath))
		})
	})

	Describe("GetUpdateSettingsPath", func() {
		It("returns a path in the sensitive settings directory if tmpfs is enabled", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "settings", "update_settings.json")

			path := platform.GetUpdateSettingsPath(true)
			Expect(path).To(Equal(expectedPath))
		})

		It("returns a path in the default directory if tmpfs is disabled", func() {
			expectedPath := filepath.Join(platform.GetDirProvider().BoshDir(), "update_settings.json")

			path := platform.GetUpdateSettingsPath(false)
			Expect(path).To(Equal(expectedPath))
		})
	})
})
