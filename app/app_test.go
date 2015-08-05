package app

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	boshterminal "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger/terminal_helpers"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() {
	Describe("App", func() {
		var (
			baseDir       string
			agentConfPath string
			agentConfJSON string
			app           App
		)

		BeforeEach(func() {
			var err error

			baseDir, err = ioutil.TempDir("", "go-agent-test")
			Expect(err).ToNot(HaveOccurred())

			err = os.Mkdir(filepath.Join(baseDir, "bosh"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())
		})

		BeforeEach(func() {
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
						"password": "some encrypted password"
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

			err := ioutil.WriteFile(settingsPath, []byte(settingsJSON), 0640)
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			err := ioutil.WriteFile(agentConfPath, []byte(agentConfJSON), 0640)
			Expect(err).ToNot(HaveOccurred())

			logger := boshlog.NewLogger(boshlog.LevelNone)
			fakefs := boshsys.NewOsFileSystem(logger)
			app = New(logger, fakefs)
		})

		AfterEach(func() {
			os.RemoveAll(baseDir)
		})

		It("Sets up device path resolver on platform specific to infrastructure", func() {
			err := app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
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
				err := app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
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
				err := app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
				Expect(err).ToNot(HaveOccurred())

				Expect(app.GetPlatform().GetDevicePathResolver()).To(
					BeAssignableToTypeOf(devicepathresolver.NewScsiDevicePathResolver(0, nil)))
			})
		})

		Context("logging stemcell version and git sha", func() {
			Context("when stemcell version and sha files are present", func() {
				It("should print out the stemcell version and sha in the logs", func() {
					stdout, _, err := boshterminal.CaptureOutputs(func() {
						logger := boshlog.NewLogger(boshlog.LevelInfo)
						fakeFs := fakesys.NewFakeFileSystem()
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_version", []byte("version-blah"))
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_git_sha1", []byte("sha1-blah"))
						app = New(logger, fakeFs)
						app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
					})

					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Running on stemcell version 'version-blah' (git: sha1-blah)"))
				})
			})

			Context("when stemcell version file is NOT present", func() {
				It("should print out the sha in the logs", func() {
					stdout, _, err := boshterminal.CaptureOutputs(func() {
						logger := boshlog.NewLogger(boshlog.LevelInfo)
						fakeFs := fakesys.NewFakeFileSystem()
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_git_sha1", []byte("sha1-blah"))
						app = New(logger, fakeFs)
						app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
					})

					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Running on stemcell version '?' (git: sha1-blah)"))
				})
			})

			Context("when sha version file is NOT present", func() {
				It("should print out the stemcell version in the logs", func() {
					stdout, _, err := boshterminal.CaptureOutputs(func() {
						logger := boshlog.NewLogger(boshlog.LevelInfo)
						fakeFs := fakesys.NewFakeFileSystem()
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_version", []byte("version-blah"))
						app = New(logger, fakeFs)
						app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
					})

					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Running on stemcell version 'version-blah' (git: ?)"))
				})
			})

			Context("when stemcell version file is empty", func() {
				It("should print out the sha in the logs", func() {
					stdout, _, err := boshterminal.CaptureOutputs(func() {
						logger := boshlog.NewLogger(boshlog.LevelInfo)
						fakeFs := fakesys.NewFakeFileSystem()
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_version", []byte(""))
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_git_sha1", []byte("sha1-blah"))
						app = New(logger, fakeFs)
						app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
					})

					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Running on stemcell version '?' (git: sha1-blah)"))
				})
			})

			Context("when sha version file is empty", func() {
				It("should print out the stemcell version in the logs", func() {
					stdout, _, err := boshterminal.CaptureOutputs(func() {
						logger := boshlog.NewLogger(boshlog.LevelInfo)
						fakeFs := fakesys.NewFakeFileSystem()
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_version", []byte("version-blah"))
						fakeFs.WriteFile("/var/vcap/bosh/etc/stemcell_git_sha1", []byte(""))
						app = New(logger, fakeFs)
						app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
					})

					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Running on stemcell version 'version-blah' (git: ?)"))
				})
			})

			Context("when stemcell version and sha files are NOT present", func() {
				It("should print unknown version and sha in the logs", func() {
					stdout, _, err := boshterminal.CaptureOutputs(func() {
						logger := boshlog.NewLogger(boshlog.LevelInfo)
						fakeFs := fakesys.NewFakeFileSystem()
						app = New(logger, fakeFs)
						app.Setup([]string{"bosh-agent", "-P", "dummy", "-C", agentConfPath, "-b", baseDir})
					})

					Expect(err).ToNot(HaveOccurred())
					Expect(stdout).To(ContainSubstring("Running on stemcell version '?' (git: ?)"))
				})
			})
		})
	})
}
