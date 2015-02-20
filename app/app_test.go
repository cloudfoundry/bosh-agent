package app

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
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
			app = New(logger)
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

				Expect(app.GetPlatform().GetDevicePathResolver()).To(
					BeAssignableToTypeOf(devicepathresolver.NewVirtioDevicePathResolver(nil, nil, boshlog.Logger{})))
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
	})
}
