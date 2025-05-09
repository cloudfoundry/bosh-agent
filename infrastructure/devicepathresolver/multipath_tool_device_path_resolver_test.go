package devicepathresolver_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("multipathDevicePathResolver", func() {
	var (
		id string

		diskSettings            boshsettings.DiskSettings
		pathResolver            DevicePathResolver
		idDevicePathResolver    *fakedpresolv.FakeDevicePathResolver
		iscsiDevicePathResolver *fakedpresolv.FakeDevicePathResolver
	)

	BeforeEach(func() {
		id = "12345678"

		idDevicePathResolver = &fakedpresolv.FakeDevicePathResolver{}
		iscsiDevicePathResolver = &fakedpresolv.FakeDevicePathResolver{}

		pathResolver = NewMultipathDevicePathResolver(idDevicePathResolver, iscsiDevicePathResolver, boshlog.NewLogger(boshlog.LevelNone))
		diskSettings = boshsettings.DiskSettings{
			ID: id,
		}
	})

	Describe("GetRealDevicePath", func() {
		Context("when id resolver get device real path", func() {
			It("returns the real path", func() {
				idDevicePathResolver.RealDevicePath = "fake-id-resolved-device-path"

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("fake-id-resolved-device-path"))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when id resolver get device real path fails", func() {
			BeforeEach(func() {
				idDevicePathResolver.GetRealDevicePathErr = errors.New("fake-resolver-err")
			})
			It("returns the real path if iSCSI resolver get device real path", func() {
				iscsiDevicePathResolver.RealDevicePath = "fake-iscsi-resolved-device-path"

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("fake-iscsi-resolved-device-path"))
				Expect(timeout).To(BeFalse())
			})

			It("returns error if iSCSI resolver get device real path fails", func() {
				iscsiDevicePathResolver.GetRealDevicePathErr = errors.New("fake-resolver-err")

				_, _, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Resolving mapped device path"))
			})
		})
	})
})
