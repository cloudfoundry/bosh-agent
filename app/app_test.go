package app

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() { //nolint:funlen,gochecknoinits
	Describe("App", func() {
		var (
			baseDir       string
			agentConfPath string
			agentConfJSON string
			app           App
			opts          Options
		)

		BeforeEach(func() {
			var err error

			baseDir, err = os.MkdirTemp("", "go-agent-test")
			Expect(err).ToNot(HaveOccurred())

			err = os.Mkdir(filepath.Join(baseDir, "bosh"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			agentConfPath = filepath.Join(baseDir, "bosh", "agent.json")

			agentConfJSON = `{
					"Infrastructure": { "Settings": { "Sources": [{ "Type": "CDROM", "FileName": "/fake-file-name" }] } }
				}`

			settingsPath := filepath.Join(baseDir, "bosh", "settings.json")

			settingsJSON := `{
				"agent_id": "my-agent-id",
				"blobstore": {
					"options": {
						"bucket_name": "george",
						"encryption_key": "optional encryption key",
						"access_key_id": "optional access key id",
						"secret_access_key": "optional secret access key"
					},
					"provider": "dummy"
				},
				"disks": {
					"ephemeral": "/dev/sdb",
					"persistent": {
						"vol-xxxxxx": "/dev/sdf"
					},
					"system": "/dev/sda1"
				},
				"env": {
					"bosh": {
						"password": "some encrypted password",
						"blobstores": [
							{
								"options": {
									"bucket_name": "2george",
									"encryption_key": "2optional encryption key",
									"access_key_id": "2optional access key id",
									"secret_access_key": "2optional secret access key",
									"port": 444
								},
								"provider": "dummy"
							},
							{
								"options": {
									"blobstore_path": "/var/vcap/micro_bosh/data/cache"
								},
								"provider": "local"
							}
						]
					}
				},
				"networks": {
					"netA": {
						"default": ["dns", "gateway"],
						"ip": "ww.ww.ww.ww",
						"dns": [
							"xx.xx.xx.xx",
							"yy.yy.yy.yy"
						]
					},
					"netB": {
						"dns": [
							"zz.zz.zz.zz"
						]
					}
				},
				"Mbus": "https://vcap:hello@0.0.0.0:6868",
				"ntp": [
					"0.north-america.pool.ntp.org",
					"1.north-america.pool.ntp.org"
				],
				"vm": {
					"name": "vm-abc-def"
				}
			}`

			err = os.WriteFile(settingsPath, []byte(settingsJSON), 0640) //nolint:gosec
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			err := os.WriteFile(agentConfPath, []byte(agentConfJSON), 0640) //nolint:gosec
			Expect(err).ToNot(HaveOccurred())

			logger := boshlog.NewLogger(boshlog.LevelNone)
			fakefs := boshsys.NewOsFileSystem(logger)
			app = New(logger, fakefs)

			opts, err = ParseOptions([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := os.RemoveAll(baseDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Sets up device path resolver on platform specific to infrastructure", func() {
			err := app.Setup(opts)
			Expect(err).ToNot(HaveOccurred())

			Expect(app.GetPlatform().GetDevicePathResolver()).To(Equal(devicepathresolver.NewIdentityDevicePathResolver()))
		})

		Context("when DevicePathResolutionType is 'virtio'", func() {
			BeforeEach(func() {
				agentConfJSON = `{
					"Platform": { "Linux": { "DevicePathResolutionType": "virtio" } },
					"Infrastructure": { "Settings": { "Sources": [{ "Type": "CDROM", "FileName": "/fake-file-name" }] } }
				}`
			})

			It("uses a VirtioDevicePathResolver", func() {
				err := app.Setup(opts)
				Expect(err).ToNot(HaveOccurred())
				logLevel, err := boshlog.Levelify("DEBUG")
				Expect(err).NotTo(HaveOccurred())

				Expect(app.GetPlatform().GetDevicePathResolver()).To(
					BeAssignableToTypeOf(devicepathresolver.NewVirtioDevicePathResolver(nil, nil, boshlog.NewLogger(logLevel))))
			})
		})

		Context("when DevicePathResolutionType is 'scsi'", func() {
			BeforeEach(func() {
				agentConfJSON = `{
					"Platform": { "Linux": { "DevicePathResolutionType": "scsi" } },
					"Infrastructure": { "Settings": { "Sources": [{ "Type": "CDROM", "FileName": "/fake-file-name" }] } }
				}`
			})

			It("uses a VirtioDevicePathResolver", func() {
				err := app.Setup(opts)
				Expect(err).ToNot(HaveOccurred())

				Expect(app.GetPlatform().GetDevicePathResolver()).To(
					BeAssignableToTypeOf(devicepathresolver.NewScsiDevicePathResolver(nil, nil, nil)))
			})
		})

		Context("logging stemcell version and git sha", func() {
			var (
				logger                  *loggerfakes.FakeLogger
				fakeFs                  boshsys.FileSystem
				stemcellVersionFilePath string
				stemcellSha1FilePath    string
			)

			JustBeforeEach(func() {
				fakeFs = fakesys.NewFakeFileSystem()
				dirProvider := boshdirs.NewProvider(baseDir)
				stemcellVersionFilePath = filepath.Join(dirProvider.EtcDir(), "stemcell_version")
				stemcellSha1FilePath = filepath.Join(dirProvider.EtcDir(), "stemcell_git_sha1")
				logger = &loggerfakes.FakeLogger{}
				app = New(logger, fakeFs)
			})

			Context("when stemcell version and sha files are present", func() {
				It("should print out the stemcell version and sha in the logs", func() {
					err := fakeFs.WriteFileString(stemcellVersionFilePath, "version-blah")
					Expect(err).NotTo(HaveOccurred())
					err = fakeFs.WriteFileString(stemcellSha1FilePath, "sha1-blah")
					Expect(err).NotTo(HaveOccurred())
					app.Setup(opts) //nolint:errcheck
					_, loggedString, _ := logger.InfoArgsForCall(0)
					Expect(loggedString).To(ContainSubstring("Running on stemcell version 'version-blah' (git: sha1-blah)"))
				})
			})

			Context("when stemcell version file is NOT present", func() {
				It("should print out the sha in the logs", func() {
					err := fakeFs.WriteFileString(stemcellSha1FilePath, "sha1-blah")
					Expect(err).NotTo(HaveOccurred())
					app.Setup(opts) //nolint:errcheck
					_, loggedString, _ := logger.InfoArgsForCall(0)
					Expect(loggedString).To(ContainSubstring("Running on stemcell version '?' (git: sha1-blah)"))
				})
			})

			Context("when sha version file is NOT present", func() {
				It("should print out the stemcell version in the logs", func() {
					err := fakeFs.WriteFileString(stemcellVersionFilePath, "version-blah")
					Expect(err).NotTo(HaveOccurred())
					app.Setup(opts) //nolint:errcheck
					_, loggedString, _ := logger.InfoArgsForCall(0)
					Expect(loggedString).To(ContainSubstring("Running on stemcell version 'version-blah' (git: ?)"))
				})
			})

			Context("when stemcell version file is empty", func() {
				It("should print out the sha in the logs", func() {
					err := fakeFs.WriteFileString(stemcellVersionFilePath, "")
					Expect(err).NotTo(HaveOccurred())
					err = fakeFs.WriteFileString(stemcellSha1FilePath, "sha1-blah")
					Expect(err).NotTo(HaveOccurred())
					app.Setup(opts) //nolint:errcheck
					_, loggedString, _ := logger.InfoArgsForCall(0)
					Expect(loggedString).To(ContainSubstring("Running on stemcell version '?' (git: sha1-blah)"))
				})
			})

			Context("when sha version file is empty", func() {
				It("should print out the stemcell version in the logs", func() {
					err := fakeFs.WriteFileString(stemcellVersionFilePath, "version-blah")
					Expect(err).NotTo(HaveOccurred())
					err = fakeFs.WriteFileString(stemcellSha1FilePath, "")
					Expect(err).NotTo(HaveOccurred())
					app.Setup(opts) //nolint:errcheck
					_, loggedString, _ := logger.InfoArgsForCall(0)
					Expect(loggedString).To(ContainSubstring("Running on stemcell version 'version-blah' (git: ?)"))
				})
			})

			Context("when stemcell version and sha files are NOT present", func() {
				It("should print unknown version and sha in the logs", func() {
					app.Setup(opts) //nolint:errcheck
					_, loggedString, _ := logger.InfoArgsForCall(0)
					Expect(loggedString).To(ContainSubstring("Running on stemcell version '?' (git: ?)"))
				})
			})
		})
	})
}
