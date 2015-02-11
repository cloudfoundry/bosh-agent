package infrastructure_test

import (
	"time"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshudev "github.com/cloudfoundry/bosh-agent/platform/udevdevice"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
)

var _ = Describe("Provider", func() {
	var (
		logger   boshlog.Logger
		platform *fakeplatform.FakePlatform
		provider Provider
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()
		options := ProviderOptions{
			StaticEphemeralDiskPath:  "fake-static-ephemeral-disk-path",
			NetworkingType:           "fake-networking-type",
			DevicePathResolutionType: "fake-device-path-resolution-type",
			Settings: SettingsOptions{
				Sources: []SourceOptions{
					CDROMSourceOptions{
						FileName: "fake-filename",
					},
				},
			},
		}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		provider = NewProvider(platform, options, logger)
	})

	Describe("Get", func() {
		It("returns infrastructure", func() {
			fs := platform.GetFs()
			udev := boshudev.NewConcreteUdevDevice(platform.GetRunner(), logger)
			idDevicePathResolver := boshdpresolv.NewIDDevicePathResolver(500*time.Millisecond, udev, fs)
			mappedDevicePathResolver := boshdpresolv.NewMappedDevicePathResolver(500*time.Millisecond, fs)

			devicePathResolvers := map[string]boshdpresolv.DevicePathResolver{
				"virtio": boshdpresolv.NewVirtioDevicePathResolver(idDevicePathResolver, mappedDevicePathResolver, logger),
				"scsi":   boshdpresolv.NewScsiDevicePathResolver(500*time.Millisecond, fs),
			}

			expectedDevicePathResolver := boshdpresolv.NewIdentityDevicePathResolver()

			cdromSettingsSource := NewCDROMSettingsSource(
				"fake-filename",
				platform,
				logger,
			)

			settingsSource, err := NewMultiSettingsSource(cdromSettingsSource)
			Expect(err).ToNot(HaveOccurred())

			expectedInf := NewGenericInfrastructure(
				platform,
				settingsSource,
				devicePathResolvers,
				expectedDevicePathResolver,

				"fake-device-path-resolution-type",
				"fake-networking-type",
				"fake-static-ephemeral-disk-path",

				logger,
			)

			inf, err := provider.Get()
			Expect(err).ToNot(HaveOccurred())
			Expect(inf).To(Equal(expectedInf))
		})
	})
})
