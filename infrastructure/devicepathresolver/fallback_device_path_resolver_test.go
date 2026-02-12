package devicepathresolver_test

import (
	"errors"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
)

var _ = Describe("FallbackDevicePathResolver", func() {
	var (
		pathResolver      DevicePathResolver
		primaryResolver   *fakedpresolv.FakeDevicePathResolver
		secondaryResolver *fakedpresolv.FakeDevicePathResolver

		diskSettings boshsettings.DiskSettings
	)

	BeforeEach(func() {
		primaryResolver = fakedpresolv.NewFakeDevicePathResolver()
		secondaryResolver = fakedpresolv.NewFakeDevicePathResolver()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		pathResolver = NewFallbackDevicePathResolver(primaryResolver, secondaryResolver, logger)

		diskSettings = boshsettings.DiskSettings{
			Lun:          "1",
			HostDeviceID: "fake-host-device-id",
		}
	})

	Describe("GetRealDevicePath", func() {
		Context("when primary resolver returns a path", func() {
			BeforeEach(func() {
				primaryResolver.RealDevicePath = "/dev/nvme0n3"
			})

			It("returns the primary path", func() {
				realPath, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(timeout).To(BeFalse())
				Expect(realPath).To(Equal("/dev/nvme0n3"))
			})

			It("does not call the secondary resolver", func() {
				_, _, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(secondaryResolver.GetRealDevicePathDiskSettings).To(Equal(boshsettings.DiskSettings{}))
			})
		})

		Context("when primary resolver errors", func() {
			BeforeEach(func() {
				primaryResolver.GetRealDevicePathErr = errors.New("symlink not found")
			})

			It("falls back to the secondary resolver", func() {
				secondaryResolver.RealDevicePath = "/dev/sdc"

				realPath, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(timeout).To(BeFalse())
				Expect(realPath).To(Equal("/dev/sdc"))

				Expect(secondaryResolver.GetRealDevicePathDiskSettings).To(Equal(diskSettings))
			})

			Context("when secondary resolver also errors", func() {
				BeforeEach(func() {
					secondaryResolver.GetRealDevicePathErr = errors.New("scsi device not found")
				})

				It("returns the secondary error wrapped", func() {
					_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Secondary resolver also failed"))
					Expect(err.Error()).To(ContainSubstring("scsi device not found"))
					Expect(timeout).To(BeFalse())
				})
			})

			Context("when secondary resolver times out", func() {
				BeforeEach(func() {
					secondaryResolver.GetRealDevicePathErr = errors.New("timed out")
					secondaryResolver.GetRealDevicePathTimedOut = true
				})

				It("returns timeout from secondary", func() {
					_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
					Expect(err).To(HaveOccurred())
					Expect(timeout).To(BeTrue())
				})
			})
		})
	})
})
