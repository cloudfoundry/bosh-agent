package agent_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	boshlogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	fakedevicepathresolver "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/disk/diskfakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager/servicemanagerfakes"

	sigar "github.com/cloudfoundry/gosigar"

	fakelogger "github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	fakeinf "github.com/cloudfoundry/bosh-agent/v2/infrastructure/fakes"
	fakedisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/v2/platform/fakes"
	fakeip "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/net/netfakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"

	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshcdrom "github.com/cloudfoundry/bosh-agent/v2/platform/cdrom"
	boshcert "github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	bosharp "github.com/cloudfoundry/bosh-agent/v2/platform/net/arp"
	boshdnsresolver "github.com/cloudfoundry/bosh-agent/v2/platform/net/dnsresolver"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshudev "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	boshsigar "github.com/cloudfoundry/bosh-agent/v2/sigar"
)

var _ = Describe("bootstrap", func() {
	Describe("Run", func() {
		var (
			platform    *platformfakes.FakePlatform
			dirProvider boshdirs.Provider
			fileSystem  *fakesys.FakeFileSystem

			settingsService *fakesettings.FakeSettingsService
			specService     *fakes.FakeV1Service

			ephemeralDiskPath string
			logger            *fakelogger.FakeLogger
		)

		BeforeEach(func() {
			platform = &platformfakes.FakePlatform{}
			dirProvider = boshdirs.NewProvider("/var/vcap")
			settingsService = &fakesettings.FakeSettingsService{
				PersistentDiskSettings: make(map[string]boshsettings.DiskSettings),
			}
			specService = fakes.NewFakeV1Service()

			ephemeralDiskPath = "/dev/sda"

			fileSystem = fakesys.NewFakeFileSystem()
			platform.GetFsReturns(fileSystem)
			platform.GetEphemeralDiskPathReturns(ephemeralDiskPath, nil)

			specService.Spec = applyspec.V1ApplySpec{
				RenderedTemplatesArchiveSpec: &applyspec.RenderedTemplatesArchiveSpec{},
				JobSpec: applyspec.JobSpec{
					JobTemplateSpecs: []applyspec.JobTemplateSpec{
						{Name: "test", Version: "1.0"},
						{Name: "second", Version: "1.0"},
					},
				},
			}
			logger = &fakelogger.FakeLogger{}
		})

		bootstrap := func() error {
			return agent.NewBootstrap(platform, dirProvider, settingsService, specService, logger).Run()
		}

		It("sets up runtime configuration", func() {
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupRuntimeConfigurationCallCount()).To(Equal(1))
		})

		It("mounts canrestart if tmpfs is enabled", func() {
			settingsService.Settings.Env.Bosh.Agent.Settings.TmpFS = true
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupCanRestartDirCallCount()).To(Equal(1))
		})

		It("does not mount canrestart if tmpfs is disabled", func() {
			settingsService.Settings.Env.Bosh.Agent.Settings.TmpFS = false
			settingsService.Settings.Env.Bosh.JobDir.TmpFS = false
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupCanRestartDirCallCount()).To(Equal(0))
		})

		Context("SSH tunnel setup for registry", func() {
			It("returns error without configuring ssh on the platform if getting public key fails", func() {
				settingsService.PublicKeyErr = errors.New("fake-get-public-key-err")

				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-public-key-err"))

				Expect(platform.SetupSSHCallCount()).To(Equal(0))
			})

			Context("when public key is not empty", func() {
				BeforeEach(func() {
					settingsService.PublicKey = "fake-public-key"
				})

				It("gets the public key and sets up ssh via the platform", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())

					Expect(platform.SetupSSHCallCount()).To(Equal(1))
					publicKey, username := platform.SetupSSHArgsForCall(0)
					Expect(publicKey).To(ConsistOf("fake-public-key"))
					Expect(username).To(Equal("vcap"))
				})

				Context("when setting up ssh fails", func() {
					BeforeEach(func() {
						platform.SetupSSHReturns(errors.New("fake-setup-ssh-err"))
					})

					It("returns an error", func() {
						err := bootstrap()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-setup-ssh-err"))
					})
				})
			})

			Context("when public key key is empty", func() {
				BeforeEach(func() {
					settingsService.PublicKey = ""
				})

				It("gets the public key and does not setup SSH", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())

					Expect(platform.SetupSSHCallCount()).To(Equal(0))
				})
			})

			Context("when the environment has authorized keys", func() {
				BeforeEach(func() {
					settingsService.Settings.Env.Bosh.AuthorizedKeys = []string{"fake-public-key", "another-fake-public-key"}
					settingsService.PublicKey = ""
				})

				It("gets the public key and sets up SSH", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())

					Expect(platform.SetupSSHCallCount()).To(Equal(1))
					publicKey, username := platform.SetupSSHArgsForCall(0)
					Expect(publicKey).To(ConsistOf("fake-public-key", "another-fake-public-key"))
					Expect(username).To(Equal("vcap"))
				})
			})

			Context("when both have authorized keys", func() {
				BeforeEach(func() {
					settingsService.Settings.Env.Bosh.AuthorizedKeys = []string{"another-fake-public-key"}
					settingsService.PublicKey = "fake-public-key"
				})

				It("gets the public key and sets up SSH", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())

					Expect(platform.SetupSSHCallCount()).To(Equal(2))
					publicKey, username := platform.SetupSSHArgsForCall(0)
					Expect(publicKey).To(ConsistOf("fake-public-key"))
					Expect(username).To(Equal("vcap"))

					publicKey, username = platform.SetupSSHArgsForCall(1)
					Expect(publicKey).To(ConsistOf("another-fake-public-key", "fake-public-key"))
					Expect(username).To(Equal("vcap"))
				})
			})
		})

		It("sets up ipv6", func() {
			settingsService.Settings.Env.Bosh.IPv6.Enable = true

			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupIPv6CallCount()).To(Equal(1))
			Expect(platform.SetupIPv6ArgsForCall(0)).To(Equal(boshsettings.IPv6{Enable: true}))
		})

		It("sets up hostname", func() {
			settingsService.Settings.AgentID = "foo-bar-baz-123"

			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupHostnameCallCount()).To(Equal(1))
			Expect(platform.SetupHostnameArgsForCall(0)).To(Equal("foo-bar-baz-123"))
		})

		It("fetches initial settings", func() {
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(settingsService.SettingsWereLoaded).To(BeTrue())
		})

		It("returns error from loading initial settings", func() {
			settingsService.LoadSettingsError = errors.New("fake-load-error")

			err := bootstrap()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-load-error"))
		})

		Context("load settings errors", func() {
			BeforeEach(func() {
				settingsService.LoadSettingsError = errors.New("fake-load-error")
				settingsService.PublicKey = "fake-public-key"
			})

			It("sets a ssh key despite settings error", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-load-error"))
				Expect(platform.SetupSSHCallCount()).To(Equal(1))
			})
		})

		It("sets up networking", func() {
			networks := boshsettings.Networks{
				"bosh": boshsettings.Network{},
			}
			mbus := "nats://user:pass@host.local"
			settingsService.Settings.Networks = networks
			settingsService.Settings.Mbus = mbus
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupNetworkingCallCount()).To(Equal(1))
			net, mbusURL := platform.SetupNetworkingArgsForCall(0)
			Expect(net).To(Equal(networks))
			Expect(mbusURL).To(Equal(mbus))

		})

		It("sets up ephemeral disk", func() {
			var swapSize uint64 = 2048
			settingsService.Settings.Env.Bosh.SwapSizeInMB = &swapSize
			settingsService.Settings.Disks = boshsettings.Disks{
				Ephemeral: "fake-ephemeral-disk-setting",
			}

			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())

			Expect(platform.SetupEphemeralDiskWithPathCallCount()).To(Equal(1))
			devicePath, desiredSwapSizeInBytes, labelPrefix := platform.SetupEphemeralDiskWithPathArgsForCall(0)
			Expect(devicePath).To(Equal("/dev/sda"))
			Expect(*desiredSwapSizeInBytes).To(Equal(uint64(2048 * 1024 * 1024)))
			Expect(labelPrefix).To(Equal(settingsService.Settings.AgentID))

			Expect(platform.GetEphemeralDiskPathCallCount()).To(Equal(1))
			Expect(platform.GetEphemeralDiskPathArgsForCall(0)).To(Equal(boshsettings.DiskSettings{
				VolumeID: "fake-ephemeral-disk-setting",
				Path:     "fake-ephemeral-disk-setting",
			}))
		})

		Context("when determining the ephemeral disk path fails", func() {
			BeforeEach(func() {
				platform.GetEphemeralDiskPathReturns("", errors.New("fake-get-ephemeral-disk-path-err"))
			})

			It("returns the error", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-ephemeral-disk-path-err"))
			})
		})

		Context("when setting up the ephemeral disk fails", func() {
			BeforeEach(func() {
				platform.SetupEphemeralDiskWithPathReturns(errors.New("fake-setup-ephemeral-disk-err"))
			})

			It("returns error if setting ephemeral disk fails", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-ephemeral-disk-err"))
			})
		})

		It("sets up raw ephemeral disks if paths exist", func() {
			diskSettings := []boshsettings.DiskSettings{{Path: "/dev/xvdb"}, {Path: "/dev/xvdc"}}

			settingsService.Settings.Disks = boshsettings.Disks{
				RawEphemeral: diskSettings,
			}

			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())

			Expect(platform.SetupRawEphemeralDisksCallCount()).To(Equal(1))
			Expect(platform.SetupRawEphemeralDisksArgsForCall(0)).To(Equal(diskSettings))
		})

		Context("when setting up the raw ephemeral disk fails", func() {
			BeforeEach(func() {
				platform.SetupRawEphemeralDisksReturns(errors.New("fake-setup-raw-ephemeral-disks-err"))
			})

			It("returns error if setting raw ephemeral disks fails", func() {
				err := bootstrap()
				Expect(platform.SetupRawEphemeralDisksCallCount()).To(Equal(1))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-raw-ephemeral-disks-err"))
			})
		})

		Describe("setting up the data dir", func() {
			It("sets up data dir", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.SetupDataDirCallCount()).To(Equal(1))
			})

			Context("when there are job directory specific feature flags", func() {
				It("passes those through to the platform", func() {
					settingsService.Settings.Env.Bosh.JobDir = boshsettings.JobDir{
						TmpFS:     true,
						TmpFSSize: "100M",
					}

					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.SetupDataDirCallCount()).To(Equal(1))
					Expect(platform.SetupDataDirArgsForCall(0)).To(Equal(boshsettings.JobDir{TmpFS: true, TmpFSSize: "100M"}))
				})
			})
		})

		Context("when setting up the data dir fails", func() {
			BeforeEach(func() {
				platform.SetupDataDirReturns(errors.New("boom"))
			})

			It("sets up data dir", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("setting up job directories", func() {
			It("sets up job dirs for all jobs", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())

				actionsCalled := specService.ActionsCalled
				Expect(actionsCalled).To(ContainElement("Get"))

				for _, jobName := range []string{"test", "second"} {
					stat := fileSystem.GetFileTestStat("/var/vcap/data/sys/log/" + jobName)
					Expect(stat).ToNot(BeNil())
					Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
					Expect(stat.Username).To(Equal("root"))
					Expect(stat.Groupname).To(Equal("vcap"))
					stat = fileSystem.GetFileTestStat("/var/vcap/data/sys/run/" + jobName)
					Expect(stat).ToNot(BeNil())
					Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
					Expect(stat.Username).To(Equal("root"))
					Expect(stat.Groupname).To(Equal("vcap"))
					stat = fileSystem.GetFileTestStat("/var/vcap/data/" + jobName)
					Expect(stat).ToNot(BeNil())
					Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
					Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
					Expect(stat.Username).To(Equal("root"))
					Expect(stat.Groupname).To(Equal("vcap"))
				}
			})

			Context("when fetching the spec from the spec service fails", func() {
				BeforeEach(func() {
					specService.GetErr = errors.New("fake-error")
				})

				It("returns an error", func() {
					err := bootstrap()
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when creating the job directories fails", func() {
				BeforeEach(func() {
					fileSystem.ChownErr = errors.New("unable to chown error")
				})

				It("returns an error", func() {
					err := bootstrap()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		It("sets up common directories", func() {
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())

			Expect(platform.SetupTmpDirCallCount()).To(Equal(1))
			Expect(platform.SetupHomeDirCallCount()).To(Equal(1))
			Expect(platform.SetupLogDirCallCount()).To(Equal(1))
			Expect(platform.SetupOptDirCallCount()).To(Equal(1))
			Expect(platform.SetupLoggingAndAuditingCallCount()).To(Equal(1))
		})

		Context("when setting up the tmp directory fails", func() {
			BeforeEach(func() {
				platform.SetupTmpDirReturns(errors.New("fake-setup-tmp-dir-err"))
			})

			It("returns an error", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-setup-tmp-dir-err"))
			})
		})

		Context("when setting up the canrestart directory fails", func() {
			BeforeEach(func() {
				settingsService.Settings.Env.Bosh.Agent.Settings.TmpFS = true
				platform.SetupCanRestartDirReturns(errors.New("fake-setup-canrestart-dir-err"))
			})

			It("returns an error", func() {
				err := bootstrap()
				Expect(err).To(MatchError("Setting up canrestart dir: fake-setup-canrestart-dir-err"))
			})
		})

		It("sets up the root disk", func() {
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupRootDiskCallCount()).To(Equal(1))
		})

		Context("when setting up the root disk fails", func() {
			BeforeEach(func() {
				platform.SetupRootDiskReturns(errors.New("growfs failed"))
			})

			It("returns an error if growing the root filesystem fails", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(platform.SetupRootDiskCallCount()).To(Equal(1))
				Expect(err.Error()).To(ContainSubstring("growfs failed"))
			})
		})

		It("sets up the RAM disk", func() {
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupSharedMemoryCallCount()).To(Equal(1))
		})

		Context("when setting up the RAM disk", func() {
			BeforeEach(func() {
				platform.SetupSharedMemoryReturns(errors.New("ramdisk-failure"))
			})

			It("returns an error if setting up the RAM disk fails", func() {
				err := bootstrap()
				Expect(err).To(HaveOccurred())
				Expect(platform.SetupSharedMemoryCallCount()).To(Equal(1))
				Expect(err.Error()).To(ContainSubstring("ramdisk-failure"))
			})
		})

		Context("setting user passwords", func() {
			BeforeEach(func() {
				settingsService.Settings.Env.Bosh.KeepRootPassword = false
				settingsService.Settings.Env.Bosh.Password = "some-encrypted-password"
			})

			It("sets root and vcap passwords", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())

				Expect(platform.SetUserPasswordCallCount()).To(Equal(2))

				username, password := platform.SetUserPasswordArgsForCall(0)
				Expect(username).To(Equal("root"))
				Expect(password).To(Equal("some-encrypted-password"))

				username, password = platform.SetUserPasswordArgsForCall(1)
				Expect(username).To(Equal("vcap"))
				Expect(password).To(Equal("some-encrypted-password"))
			})

			Context("when keep_root_password is set", func() {
				BeforeEach(func() {
					settingsService.Settings.Env.Bosh.KeepRootPassword = true
				})

				It("does not change root password if keep_root_password is set to true", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())

					Expect(platform.SetUserPasswordCallCount()).To(Equal(1))

					username, password := platform.SetUserPasswordArgsForCall(0)
					Expect(username).To(Equal("vcap"))
					Expect(password).To(Equal("some-encrypted-password"))
				})
			})
		})

		It("setups up monit", func() {
			err := bootstrap()
			Expect(err).NotTo(HaveOccurred())
			Expect(platform.SetupMonitUserCallCount()).To(Equal(1))
			Expect(platform.StartMonitCallCount()).To(Equal(1))
		})

		Context("when RemoveDevTools is requested", func() {
			BeforeEach(func() {
				settingsService.Settings.Env.Bosh.RemoveDevTools = true
			})

			It("removes development tools", func() {
				err := platform.GetFs().WriteFileString(path.Join(dirProvider.EtcDir(), "dev_tools_file_list"), "/usr/bin/gfortran")
				Expect(err).NotTo(HaveOccurred())

				err = bootstrap()
				Expect(err).NotTo(HaveOccurred())
				Expect(platform.RemoveDevToolsCallCount()).To(Equal(1))
				Expect(platform.RemoveDevToolsArgsForCall(0)).To(Equal(
					path.Join(dirProvider.EtcDir(), "dev_tools_file_list"),
				))
			})

			Context("when dev_tools_file_list does not exist", func() {
				It("does nothing", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.RemoveDevToolsCallCount()).To(Equal(0))
				})
			})
		})

		Context("when RemoveStaticLibraries is requested", func() {
			BeforeEach(func() {
				settingsService.Settings.Env.Bosh.RemoveStaticLibraries = true
			})

			Context("and the static libraries path exists", func() {
				BeforeEach(func() {
					err := platform.GetFs().WriteFileString(path.Join(dirProvider.EtcDir(), "static_libraries_list"), "/usr/lib/libsupp.a")
					Expect(err).NotTo(HaveOccurred())
				})

				It("removes static libraries", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.RemoveStaticLibrariesCallCount()).To(Equal(1))
					Expect(platform.RemoveStaticLibrariesArgsForCall(0)).To(Equal(
						path.Join(dirProvider.EtcDir(), "static_libraries_list"),
					))
				})
			})

			Context("and the static libraries path does not exist", func() {
				It("does nothing", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.RemoveStaticLibrariesCallCount()).To(Equal(0))
				})
			})
		})

		Context("when ntp servers are configured", func() {
			var ntpServers []string

			BeforeEach(func() {
				ntpServers = []string{
					"0.north-america.pool.ntp.org",
					"1.north-america.pool.ntp.org",
				}
				settingsService.Settings.NTP = ntpServers
				settingsService.Settings.Env.Bosh.NTP = nil
			})

			It("sets ntp", func() {
				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())

				Expect(platform.SetTimeWithNtpServersCallCount()).To(Equal(1))
				Expect(platform.SetTimeWithNtpServersArgsForCall(0)).To(Equal(ntpServers))
			})

			Context("when ntp is set on the bosh env", func() {
				var anotherNtpServers []string
				BeforeEach(func() {
					anotherNtpServers = []string{
						"2.north-america.pool.ntp.org",
						"3.north-america.pool.ntp.org",
					}
					settingsService.Settings.Env.Bosh.NTP = anotherNtpServers
				})

				It("sets ntp with the servers from bosh env", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())

					Expect(platform.SetTimeWithNtpServersCallCount()).To(Equal(1))
					Expect(platform.SetTimeWithNtpServersArgsForCall(0)).To(Equal(anotherNtpServers))
				})
			})

			It("sets up the log directories before calling SetTimeWithNTPServers", func() {
				logNeverCalled := fmt.Errorf("SetupLogDir was never called")
				platform.SetTimeWithNtpServersStub = func([]string) error {
					return logNeverCalled
				}
				platform.SetupLogDirStub = func() error {
					logNeverCalled = nil
					return nil
				}

				err := bootstrap()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("validating persistent disks", func() {
			Context("when UpdateSettings contains DiskAssociations", func() {
				BeforeEach(func() {
					settingsService.Settings.UpdateSettings = boshsettings.UpdateSettings{
						DiskAssociations: []boshsettings.DiskAssociation{
							{
								Name:    "test-disk",
								DiskCID: "vol-123",
							},
							{
								Name:    "test-disk-2",
								DiskCID: "vol-456",
							},
						},
					}

					settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
						"vol-123": {
							Path: "/dev/sdb",
							ID:   "vol-123",
						},
						"vol-456": {
							Path: "/dev/sdc",
							ID:   "vol-456",
						},
					}
				})

				It("successfully bootstraps", func() {
					err := bootstrap()
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when the disk associations are not provided as attached disks", func() {
					BeforeEach(func() {
						settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{}
						settingsService.GetPersistentDiskSettingsError = errors.New("disk not found")
					})

					It("returns an error", func() {
						err := bootstrap()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Disk vol-123 is not attached"))
					})
				})

				Context("when there are attached disks that do not have disk associations", func() {
					BeforeEach(func() {
						settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
							"vol-123": {
								Path: "/dev/sdb",
								ID:   "vol123",
							},
							"vol-456": {
								Path: "/dev/sdc",
								ID:   "vol123",
							},
							"vol-789": {
								Path: "/dev/sdd",
								ID:   "vol123",
							},
						}
					})

					It("returns an error", func() {
						err := bootstrap()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Unexpected disk attached"))
					})
				})
			})

			Context("when UpdateSettings does not contain DiskAssociations", func() {
				Context("when there are no managed persistent disks provided", func() {
					It("does not error", func() {
						err := bootstrap()
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("when there is a single managed persistent disk provided", func() {
					BeforeEach(func() {
						settingsService.Settings.Disks = boshsettings.Disks{
							Persistent: map[string]interface{}{
								"vol-123": "/dev/sdb",
							},
						}
					})

					It("does not error", func() {
						err := bootstrap()
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("when there are more than one persistent disks provided", func() {
					BeforeEach(func() {
						settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
							"vol-123": {
								Path: "/dev/sdb",
								ID:   "vol-123",
							},
							"vol-456": {
								Path: "/dev/sdc",
								ID:   "vol-123",
							},
						}
					})

					It("returns an error", func() {
						err := bootstrap()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Unexpected disk attached"))
					})
				})

				Context("when managed_disk_settings.json exists", func() {
					var managedDiskSettingsPath string

					BeforeEach(func() {
						diskCid := "i-am-a-disk-cid"

						managedDiskSettingsPath = filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")
						err := fileSystem.WriteFile(managedDiskSettingsPath, []byte(diskCid))
						Expect(err).NotTo(HaveOccurred())

						updateSettings := boshsettings.UpdateSettings{
							DiskAssociations: []boshsettings.DiskAssociation{
								{
									Name:    "test-disk",
									DiskCID: diskCid,
								},
							},
						}
						updateSettingsBytes, err := json.Marshal(updateSettings)
						Expect(err).ToNot(HaveOccurred())

						updateSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "update_settings.json")
						err = fileSystem.WriteFile(updateSettingsPath, updateSettingsBytes)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("and the provided disk CID is the same", func() {
						BeforeEach(func() {
							settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
								"i-am-a-disk-cid": {
									Path: "/dev/sdb",
									ID:   "i-am-a-disk-cid",
								},
							}
						})

						It("does not error", func() {
							err := bootstrap()
							Expect(err).ToNot(HaveOccurred())
						})
					})

					Context("and the provided disk CID is not the same", func() {
						BeforeEach(func() {
							settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
								"i-am-a-different-disk-cid": {
									Path: "/dev/sdb",
									ID:   "i-am-a-disk-cid",
								},
							}
							settingsService.GetPersistentDiskSettingsError = errors.New("disk not found")
						})

						It("returns an error", func() {
							err := bootstrap()
							Expect(err).To(HaveOccurred())

							Expect(err.Error()).To(ContainSubstring("Attached disk disagrees with previous mount: disk not found"))
						})
					})

					Context("when there are no provided disks", func() {
						It("does not return an error", func() {
							err := bootstrap()
							Expect(err).NotTo(HaveOccurred())
						})
					})

					Context("when reading the managed_disk_settings.json errors", func() {
						BeforeEach(func() {
							fileSystem.RegisterReadFileError(managedDiskSettingsPath, errors.New("oh noes"))
						})

						It("returns an error", func() {
							err := bootstrap()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("Reading managed_disk_settings.json"))
						})
					})
				})

				Context("and no disks are provided", func() {
					BeforeEach(func() {
						settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{}
					})

					It("successfully bootstraps", func() {
						err := bootstrap()
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("and there are disks provided", func() {
					BeforeEach(func() {
						settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
							"vol-123": {
								Path: "/dev/sdb",
								ID:   "vol-123",
							},
							"vol-456": {
								Path: "/dev/sdc",
								ID:   "vol-456",
							},
						}
					})

					It("returns error", func() {
						err := bootstrap()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Unexpected disk attached"))
					})
				})
			})
		})

		Context("mounting persistent disks", func() {
			BeforeEach(func() {
				updateSettings := boshsettings.UpdateSettings{}
				updateSettingsBytes, err := json.Marshal(updateSettings)
				Expect(err).ToNot(HaveOccurred())

				updateSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "update_settings.json")
				err = fileSystem.WriteFile(updateSettingsPath, updateSettingsBytes)
				Expect(err).NotTo(HaveOccurred())

				settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{
					"vol-123": {
						VolumeID: "2",
						Path:     "/dev/sdb",
						ID:       "vol-123",
					},
				}

				platform.IsPersistentDiskMountableReturns(true, nil)
			})
			Context("when adjusting persistent disk partitioning fails", func() {
				BeforeEach(func() {
					diskCid := "vol-123"
					managedDiskSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")
					err := fileSystem.WriteFile(managedDiskSettingsPath, []byte(diskCid))
					Expect(err).NotTo(HaveOccurred())

					platform.AdjustPersistentDiskPartitioningReturns(errors.New("grow partition fail"))
				})
				It("should return error", func() {
					err := bootstrap()
					Expect(err).To(HaveOccurred())
					Expect(platform.AdjustPersistentDiskPartitioningCallCount()).To(Equal(1))
					Expect(platform.MountPersistentDiskCallCount()).To(Equal(0))
					Expect(err.Error()).To(Equal("Mounting last mounted disk: Adjusting persistent disk partitioning: grow partition fail"))
				})
			})

			Context("when mounting persistent disk fails", func() {
				BeforeEach(func() {
					diskCid := "vol-123"
					managedDiskSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")
					err := fileSystem.WriteFile(managedDiskSettingsPath, []byte(diskCid))
					Expect(err).NotTo(HaveOccurred())

					platform.MountPersistentDiskReturns(errors.New("mount fail"))
				})
				It("should return error", func() {
					err := bootstrap()
					Expect(err).To(HaveOccurred())
					Expect(platform.MountPersistentDiskCallCount()).To(Equal(1))
					Expect(err.Error()).To(Equal("Mounting last mounted disk: Mounting persistent disk: mount fail"))
				})
			})

			Context("when checking if the persistent disk is mountable fails", func() {
				BeforeEach(func() {
					diskCid := "vol-123"
					managedDiskSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")
					err := fileSystem.WriteFile(managedDiskSettingsPath, []byte(diskCid))
					Expect(err).NotTo(HaveOccurred())

					platform.IsPersistentDiskMountableReturns(false, errors.New("boom"))
				})

				It("returns an error", func() {
					err := bootstrap()
					Expect(err).To(HaveOccurred())
					Expect(platform.MountPersistentDiskCallCount()).To(Equal(0))
					Expect(err.Error()).To(Equal("Mounting last mounted disk: Checking if persistent disk is partitioned: boom"))
				})

			})

			Context("when there are no persistent disks", func() {
				BeforeEach(func() {
					settingsService.PersistentDiskSettings = map[string]boshsettings.DiskSettings{}
				})

				It("does not attempt to mount persistent disks", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.MountPersistentDiskCallCount()).To(Equal(0))
				})
			})

			Context("when the last mounted cid information is not present", func() {
				It("does not try to mount the persistent disk", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.MountPersistentDiskCallCount()).To(Equal(0))
				})
			})

			Context("when the last mounted cid information is present", func() {
				BeforeEach(func() {
					diskCid := "vol-123"
					managedDiskSettingsPath := filepath.Join(platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")
					err := fileSystem.WriteFile(managedDiskSettingsPath, []byte(diskCid))
					Expect(err).NotTo(HaveOccurred())
				})

				It("mounts persistent disk", func() {
					err := bootstrap()
					Expect(err).NotTo(HaveOccurred())
					Expect(platform.MountPersistentDiskCallCount()).To(Equal(1))
					diskSettings, storeDir := platform.MountPersistentDiskArgsForCall(0)
					Expect(diskSettings).To(Equal(boshsettings.DiskSettings{
						ID:       "vol-123",
						VolumeID: "2",
						Path:     "/dev/sdb",
					}))
					Expect(storeDir).To(Equal(dirProvider.StoreDir()))
				})

				Context("and the persistent disk is not mountable", func() {
					BeforeEach(func() {
						platform.IsPersistentDiskMountableReturns(false, nil)
					})

					It("does not try to mount the persistent disk", func() {
						err := bootstrap()
						Expect(err).NotTo(HaveOccurred())
						Expect(platform.MountPersistentDiskCallCount()).To(Equal(0))
					})
				})

				Context("and cannot find persistent disk", func() {
					BeforeEach(func() {
						settingsService.GetPersistentDiskSettingsError = errors.New("disk not found")
					})

					It("returns and error", func() {
						err := bootstrap()
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})

		Describe("Network setup exercised by Run", func() {
			var (
				settingsJSON string

				fs                     *fakesys.FakeFileSystem
				platform               boshplatform.Platform
				boot                   agent.Bootstrap
				defaultNetworkResolver boshsettings.DefaultNetworkResolver
				logger                 boshlog.Logger
				dirProvider            boshdirs.Provider
				specService            *fakes.FakeV1Service
				serviceManager         servicemanagerfakes.FakeServiceManager

				interfaceAddrsProvider *fakeip.FakeInterfaceAddressesProvider
				fakeMACAddressDetector *netfakes.FakeMACAddressDetector
			)

			stubInterfaces := func(interfaces [][]string) {
				addresses := map[string]string{}
				for _, iface := range interfaces {
					addresses[iface[1]] = iface[0]
				}

				fakeMACAddressDetector.DetectMacAddressesReturns(addresses, nil)
			}

			BeforeEach(func() {
				fs = fakesys.NewFakeFileSystem()
				specService = fakes.NewFakeV1Service()
				runner := fakesys.NewFakeCmdRunner()
				dirProvider = boshdirs.NewProvider("/var/vcap/bosh")
				serviceManager = servicemanagerfakes.FakeServiceManager{}

				linuxOptions := boshplatform.LinuxOptions{
					CreatePartitionIfNoEphemeralDisk: true,
				}

				mountSearcher := &fakedisk.FakeMountsSearcher{}
				mountSearcher.SearchMountsMounts = []boshdisk.Mount{
					{MountPoint: "/", PartitionPath: "rootfs"},
					{MountPoint: "/", PartitionPath: "/dev/vda1"},
				}

				rootDevicePartitioner := fakedisk.NewFakePartitioner()
				rootDevicePartitioner.GetDeviceSizeInBytesSizes["/dev/vda"] = 1024 * 1024 * 1024

				formatter := &fakedisk.FakeFormatter{}
				mounter := &diskfakes.FakeMounter{}

				diskManager := &diskfakes.FakeManager{}
				diskManager.GetMountsSearcherReturns(mountSearcher)
				diskManager.GetRootDevicePartitionerReturns(rootDevicePartitioner)
				diskManager.GetFormatterReturns(formatter)
				diskManager.GetMounterReturns(mounter)

				// for the GrowRootFS call to findRootDevicePath
				runner.AddCmdResult(
					"readlink -f /dev/vda1",
					fakesys.FakeCmdResult{Stdout: "/dev/vda1"},
				)

				// for the createEphemeralPartitionsOnRootDevice call to findRootDevicePath
				runner.AddCmdResult(
					"readlink -f /dev/vda1",
					fakesys.FakeCmdResult{Stdout: "/dev/vda1"},
				)

				logger = boshlog.NewLogger(boshlog.LevelNone)
				udev := boshudev.NewConcreteUdevDevice(runner, logger)
				linuxCdrom := boshcdrom.NewLinuxCdrom("/dev/sr0", udev, runner)
				linuxCdutil := boshcdrom.NewCdUtil(dirProvider.SettingsDir(), fs, linuxCdrom, logger)

				compressor := boshcmd.NewTarballCompressor(runner, fs)
				copier := boshcmd.NewGenericCpCopier(fs, logger)
				logsTarProvider := boshlogstarprovider.NewLogsTarProvider(compressor, copier, dirProvider)

				sigarCollector := boshsigar.NewSigarStatsCollector(&sigar.ConcreteSigar{})

				vitalsService := boshvitals.NewService(sigarCollector, dirProvider, mounter)

				ipResolver := boship.NewResolver(boship.NetworkInterfaceToAddrsFunc)

				arping := bosharp.NewArping(runner, fs, logger, boshplatform.ArpIterations, boshplatform.ArpIterationDelay, boshplatform.ArpInterfaceCheckDelay)
				interfaceConfigurationCreator := boshnet.NewInterfaceConfigurationCreator(logger)

				interfaceAddrsProvider = &fakeip.FakeInterfaceAddressesProvider{}
				kernelIPv6 := boshnet.NewKernelIPv6Impl(fs, runner, logger)
				fakeMACAddressDetector = &netfakes.FakeMACAddressDetector{}
				err := fs.WriteFileString("/etc/resolv.conf", "8.8.8.8 4.4.4.4")
				Expect(err).NotTo(HaveOccurred())

				dnsResolver := boshdnsresolver.NewResolveConfResolver(fs, runner)

				ubuntuNetManager := boshnet.NewUbuntuNetManager(fs, runner, ipResolver, fakeMACAddressDetector, interfaceConfigurationCreator, interfaceAddrsProvider, dnsResolver, arping, kernelIPv6, logger)
				ubuntuCertManager := boshcert.NewUbuntuCertManager(fs, runner, 1, logger)

				monitRetryable := boshplatform.NewMonitRetryable(&serviceManager)
				monitRetryStrategy := boshretry.NewAttemptRetryStrategy(10, 1*time.Second, monitRetryable, logger)

				devicePathResolver := fakedevicepathresolver.NewFakeDevicePathResolver()

				fakeUUIDGenerator := boshuuid.NewGenerator()
				routesSearcher := boshnet.NewRoutesSearcher(logger, runner, nil)
				defaultNetworkResolver = boshnet.NewDefaultNetworkResolver(routesSearcher, ipResolver)
				state, err := boshplatform.NewBootstrapState(fs, "/tmp/agent_state.json")
				Expect(err).NotTo(HaveOccurred())

				platform = boshplatform.NewLinuxPlatform(
					fs,
					runner,
					sigarCollector,
					compressor,
					copier,
					dirProvider,
					vitalsService,
					linuxCdutil,
					diskManager,
					ubuntuNetManager,
					ubuntuCertManager,
					monitRetryStrategy,
					devicePathResolver,
					state,
					linuxOptions,
					logger,
					defaultNetworkResolver,
					fakeUUIDGenerator,
					boshplatform.NewDelayedAuditLogger(fakeplatform.NewFakeAuditLoggerProvider(), logger),
					logsTarProvider,
					&serviceManager,
				)
			})

			JustBeforeEach(func() {
				var settings boshsettings.Settings
				err := json.Unmarshal([]byte(settingsJSON), &settings)
				Expect(err).NotTo(HaveOccurred())

				settingsSource := fakeinf.FakeSettingsSource{
					PublicKey:     "123",
					SettingsValue: settings,
				}

				settingsService := boshsettings.NewService(
					platform.GetFs(),
					settingsSource,
					platform,
					logger,
				)

				boot = agent.NewBootstrap(
					platform,
					dirProvider,
					settingsService,
					specService,
					logger,
				)
			})

			Context("when a single network configuration is provided, with a MAC address", func() {
				BeforeEach(func() {
					settingsJSON = `{
								"networks": {
									"netA": {
										"default": ["dns", "gateway"],
										"ip": "2.2.2.2",
										"dns": [
											"8.8.8.8",
											"4.4.4.4"
										],
										"netmask": "255.255.255.0",
										"gateway": "2.2.2.0",
										"mac": "aa:bb:cc"
									}
								}
							}`
				})

				Context("and a single physical network interface exists", func() {
					BeforeEach(func() {
						stubInterfaces([][]string{{"eth0", "aa:bb:cc"}})
						interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
							boship.NewSimpleInterfaceAddress("eth0", "2.2.2.2"),
						}
					})

					It("succeeds", func() {
						err := boot.Run()
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("and extra physical network interfaces exist", func() {
					BeforeEach(func() {
						stubInterfaces([][]string{{"eth0", "aa:bb:cc"}, {"eth1", "aa:bb:dd"}})
						interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
							boship.NewSimpleInterfaceAddress("eth0", "2.2.2.2"),
						}
					})

					It("succeeds", func() {
						err := boot.Run()
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("when a single network configuration is provided, without a MAC address", func() {
				BeforeEach(func() {
					settingsJSON = `{
								"networks": {
									"netA": {
										"default": ["dns", "gateway"],
										"ip": "2.2.2.2",
										"dns": [
											"8.8.8.8",
											"4.4.4.4"
										],
										"netmask": "255.255.255.0",
										"gateway": "2.2.2.0"
									}
								}
							}`
				})

				Context("and a single physical network interface exists", func() {
					BeforeEach(func() {
						stubInterfaces([][]string{{"eth0", "aa:bb:cc"}})
						interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
							boship.NewSimpleInterfaceAddress("eth0", "2.2.2.2"),
						}
					})

					It("succeeds", func() {
						err := boot.Run()
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("and extra physical network interfaces exist", func() {
					BeforeEach(func() {
						stubInterfaces([][]string{{"eth0", "aa:bb:cc"}, {"eth1", "aa:bb:dd"}})
						interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
							boship.NewSimpleInterfaceAddress("eth0", "2.2.2.2"),
						}
					})

					It("succeeds", func() {
						err := boot.Run()
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("when two network configurations are provided", func() {
				BeforeEach(func() {
					settingsJSON = `{
								"networks": {
									"netA": {
										"default": ["dns", "gateway"],
										"ip": "2.2.2.2",
										"dns": [
											"8.8.8.8",
											"4.4.4.4"
										],
										"netmask": "255.255.255.0",
										"gateway": "2.2.2.0",
										"mac": "aa:bb:cc"
									},
									"netB": {
										"default": ["dns", "gateway"],
										"ip": "3.3.3.3",
										"dns": [
											"8.8.8.8",
											"4.4.4.4"
										],
										"netmask": "255.255.255.0",
										"gateway": "3.3.3.0",
										"mac": ""
									}
								}
							}`
				})

				Context("and a single physical network interface exists", func() {
					BeforeEach(func() {
						stubInterfaces([][]string{{"eth0", "aa:bb:cc"}})
						interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
							boship.NewSimpleInterfaceAddress("eth0", "2.2.2.2"),
						}
					})

					It("succeeds", func() {
						err := boot.Run()
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("and two physical network interfaces with matching MAC addresses exist", func() {
					BeforeEach(func() {
						stubInterfaces([][]string{{"eth0", "aa:bb:cc"}, {"eth1", "aa:bb:dd"}})
						interfaceAddrsProvider.GetInterfaceAddresses = []boship.InterfaceAddress{
							boship.NewSimpleInterfaceAddress("eth0", "2.2.2.2"),
							boship.NewSimpleInterfaceAddress("eth1", "3.3.3.3"),
						}
					})

					It("succeeds", func() {
						err := boot.Run()
						Expect(err).ToNot(HaveOccurred())
					})
				})
			})
		})
	})
})
