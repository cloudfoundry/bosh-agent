package platform_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider/logstarproviderfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/fakes"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
	fakestats "github.com/cloudfoundry/bosh-agent/v2/platform/stats/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

type mount struct {
	MountDir string
	DiskCid  string
}

var _ = Describe("DummyPlatform", func() {
	var (
		platform           Platform
		collector          boshstats.Collector
		fs                 *fakesys.FakeFileSystem
		cmdRunner          boshsys.CmdRunner
		dirProvider        boshdirs.Provider
		devicePathResolver boshdpresolv.DevicePathResolver
		logger             boshlog.Logger
		auditLogger        AuditLogger
		logsTarProvider    *logstarproviderfakes.FakeLogsTarProvider
	)

	BeforeEach(func() {
		collector = &fakestats.FakeCollector{}
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		dirProvider = boshdirs.NewProvider("/fake-dir")
		devicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		auditLogger = fakes.NewFakeAuditLogger()
		logsTarProvider = &logstarproviderfakes.FakeLogsTarProvider{}
	})

	JustBeforeEach(func() {
		platform = NewDummyPlatform(
			collector,
			fs,
			cmdRunner,
			dirProvider,
			devicePathResolver,
			logger,
			auditLogger,
			logsTarProvider,
		)
	})

	Describe("GetDefaultNetwork", func() {
		It("returns the contents of dummy-defaults-network-settings.json since that's what the dummy cpi writes", func() {
			settingsFilePath := "/fake-dir/bosh/dummy-default-network-settings.json"
			err := fs.WriteFileString(settingsFilePath, `{"IP": "1.2.3.4"}`)
			Expect(err).NotTo(HaveOccurred())

			network, err := platform.GetDefaultNetwork(boship.IPv4)
			Expect(err).NotTo(HaveOccurred())

			Expect(network.IP).To(Equal("1.2.3.4"))
		})
	})

	Describe("GetCertManager", func() {
		It("returns a dummy cert manager", func() {
			certManager := platform.GetCertManager()

			Expect(certManager.UpdateCertificates("")).Should(BeNil())
		})
	})

	Describe("MountPersistentDisk", func() {
		var diskSettings boshsettings.DiskSettings
		var mountsPath, managedSettingsPath, formattedDisksPath string

		BeforeEach(func() {
			diskSettings = boshsettings.DiskSettings{ID: "somediskid", MountOptions: []string{"mountOption1", "mountOption2"}}
			mountsPath = filepath.Join(dirProvider.BoshDir(), "mounts.json")
			managedSettingsPath = filepath.Join(dirProvider.BoshDir(), "managed_disk_settings.json")
			formattedDisksPath = filepath.Join(dirProvider.BoshDir(), "formatted_disks.json")
		})

		It("Mounts a persistent disk", func() {
			mountsContent, _ := fs.ReadFileString(mountsPath) //nolint:errcheck
			Expect(mountsContent).To(Equal(""))

			err := platform.MountPersistentDisk(diskSettings, "/dev/potato")
			Expect(err).NotTo(HaveOccurred())

			mountsContent, _ = fs.ReadFileString(mountsPath) //nolint:errcheck
			Expect(mountsContent).To(Equal(`[{"MountDir":"/dev/potato","MountOptions":["mountOption1","mountOption2"],"DiskCid":"somediskid"}]`))
		})

		It("Updates the managed disk settings", func() {
			lastMountedCid, _ := fs.ReadFileString(managedSettingsPath) //nolint:errcheck
			Expect(lastMountedCid).To(Equal(""))

			err := platform.MountPersistentDisk(diskSettings, "/dev/potato")
			Expect(err).NotTo(HaveOccurred())

			lastMountedCid, _ = fs.ReadFileString(managedSettingsPath) //nolint:errcheck
			Expect(lastMountedCid).To(Equal("somediskid"))
		})

		It("Updates the formatted disks", func() {
			formattedDisks, _ := fs.ReadFileString(formattedDisksPath) //nolint:errcheck
			Expect(formattedDisks).To(Equal(""))

			err := platform.MountPersistentDisk(diskSettings, "/dev/potato")
			Expect(err).NotTo(HaveOccurred())

			formattedDisks, _ = fs.ReadFileString(formattedDisksPath) //nolint:errcheck
			Expect(formattedDisks).To(Equal(`[{"DiskCid":"somediskid"}]`))
		})

		Context("Device has already been mounted as expected", func() {
			BeforeEach(func() {
				err := fs.WriteFileString(managedSettingsPath, "somediskid")
				Expect(err).NotTo(HaveOccurred())
				err = fs.WriteFileString(mountsPath, `[{"MountDir":"/dev/potato","DiskCid":"somediskid"}]`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("Does not mount in new location", func() {
				err := platform.MountPersistentDisk(diskSettings, "/dev/potato")
				Expect(err).NotTo(HaveOccurred())

				mountsContent, _ := fs.ReadFileString(mountsPath) //nolint:errcheck
				Expect(mountsContent).To(Equal(`[{"MountDir":"/dev/potato","DiskCid":"somediskid"}]`))
			})
		})
	})

	Describe("UnmountPersistentDisk", func() {
		Context("when there are two mounted persistent disks in the mounts json", func() {
			BeforeEach(func() {
				var mounts []mount
				mounts = append(mounts, mount{MountDir: "dir1", DiskCid: "cid1"})
				mounts = append(mounts, mount{MountDir: "dir2", DiskCid: "cid2"})
				mountsJSON, err := json.Marshal(mounts)
				Expect(err).NotTo(HaveOccurred())

				mountsPath := filepath.Join(dirProvider.BoshDir(), "mounts.json")
				err = fs.WriteFile(mountsPath, mountsJSON)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes one of the disks from the mounts json", func() {
				unmounted, err := platform.UnmountPersistentDisk(boshsettings.DiskSettings{ID: "cid1"})
				Expect(err).NotTo(HaveOccurred())
				Expect(unmounted).To(Equal(true))

				_, isMountPoint, err := platform.IsMountPoint("dir1")
				Expect(err).NotTo(HaveOccurred())
				Expect(isMountPoint).To(Equal(false))

				_, isMountPoint, err = platform.IsMountPoint("dir2")
				Expect(err).NotTo(HaveOccurred())
				Expect(isMountPoint).To(Equal(true))
			})
		})
	})

	Describe("SetDiskAssociations", func() {
		It("writes the associations to the file", func() {
			diskName1 := "disk1"
			diskName2 := "disk2"

			err := platform.AssociateDisk(diskName1, boshsettings.DiskSettings{})
			Expect(err).NotTo(HaveOccurred())

			err = platform.AssociateDisk(diskName2, boshsettings.DiskSettings{})
			Expect(err).NotTo(HaveOccurred())
			diskAssociationsPath := filepath.Join(dirProvider.BoshDir(), "disk_associations.json")

			actualDiskNames := []string{}
			fileContent, err := fs.ReadFile(diskAssociationsPath)
			Expect(err).NotTo(HaveOccurred())

			err = json.Unmarshal(fileContent, &actualDiskNames)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualDiskNames).To(ConsistOf([]string{
				diskName1,
				diskName2,
			}))
		})
	})

	Describe("IsPersistentDiskMountable", func() {
		BeforeEach(func() {
			formattedDisksPath := filepath.Join(dirProvider.BoshDir(), "formatted_disks.json")
			err := fs.WriteFileString(formattedDisksPath, `[{"DiskCid": "my-disk-id"}]`)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when disk has been formatted", func() {
			It("returns true with no error", func() {
				diskSettings := boshsettings.DiskSettings{ID: "my-disk-id"}

				mountable, err := platform.IsPersistentDiskMountable(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(mountable).To(Equal(true))
			})
		})

		Context("when disk has NOT been formatted", func() {
			It("returns false with no error", func() {
				diskSettings := boshsettings.DiskSettings{ID: "some-other-disk-id"}

				mountable, err := platform.IsPersistentDiskMountable(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(mountable).To(Equal(false))
			})
		})
	})

	Describe("SetUserPassword", func() {
		It("writes the password to a file", func() {
			err := platform.SetUserPassword("user-name", "fake-password")
			Expect(err).NotTo(HaveOccurred())

			userPasswordsPath := filepath.Join(dirProvider.BoshDir(), "user-name", CredentialFileName)
			password, err := fs.ReadFileString(userPasswordsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(password).To(Equal("fake-password"))
		})

		It("writes the passwords to different files for each user", func() {
			err := platform.SetUserPassword("user-name1", "fake-password1")
			Expect(err).NotTo(HaveOccurred())
			err = platform.SetUserPassword("user-name2", "fake-password2")
			Expect(err).NotTo(HaveOccurred())

			userPasswordsPath := filepath.Join(dirProvider.BoshDir(), "user-name1", CredentialFileName)
			password, err := fs.ReadFileString(userPasswordsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(password).To(Equal("fake-password1"))

			userPasswordsPath = filepath.Join(dirProvider.BoshDir(), "user-name2", CredentialFileName)
			password, err = fs.ReadFileString(userPasswordsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(password).To(Equal("fake-password2"))
		})
	})

	Describe("SetupDataDir", func() {
		It("creates a link from BASEDIR/sys to BASEDIR/data/sys", func() {
			err := platform.SetupDataDir(boshsettings.JobDir{}, boshsettings.RunDir{})
			Expect(err).NotTo(HaveOccurred())

			stat := fs.GetFileTestStat(filepath.Clean("/fake-dir/sys"))

			Expect(stat).ToNot(BeNil())
			Expect(stat.SymlinkTarget).To(Equal("/fake-dir/data/sys"))
		})
	})

	Describe("SetupBlobsDir", func() {
		It("creates a blobs folder under BASEDIR/DATADIR with correct permissions", func() {
			err := platform.SetupBlobsDir()
			Expect(err).NotTo(HaveOccurred())

			stat := fs.GetFileTestStat(filepath.Clean("/fake-dir/data/blobs"))

			Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(stat.FileMode).To(Equal(os.FileMode(0700)))
		})
	})

	Describe("SetupBoshSettingsDisk", func() {
		It("creates the sensitive directory for the agent settings file with correct permissions", func() {
			err := platform.SetupBoshSettingsDisk()
			Expect(err).NotTo(HaveOccurred())

			stat := fs.GetFileTestStat(filepath.Clean("/fake-dir/bosh/settings"))

			Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(stat.FileMode).To(Equal(os.FileMode(0700)))

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
