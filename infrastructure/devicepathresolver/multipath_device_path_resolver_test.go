package devicepathresolver_test

import (
	"time"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("multipathDevicePathResolver", func() {
	var (
		diskSettings boshsettings.DiskSettings
		resolver     DevicePathResolver
		usePreformattedPersistentDisk bool
	)

	BeforeEach(func() {
		diskSettings = boshsettings.DiskSettings{
			Path: "/fake/device/path",
		}
	})

	Context("when usePreformattedPersistentDisk is true", func() {
		BeforeEach(func() {
				usePreformattedPersistentDisk = true
			resolver = NewMultipathDevicePathResolver(usePreformattedPersistentDisk)
		})

		It("returns the path directly", func() {
			realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(timedOut).To(BeFalse())
			Expect(realPath).To(Equal("/fake/device/path"))
		})
	})

	Context("when usePreformattedPersistentDisk is false", func() {
		BeforeEach(func() {
				usePreformattedPersistentDisk = false
			resolver = NewMultipathDevicePathResolver(usePreformattedPersistentDisk)
		})

		It("returns the path + -part", func() {
			realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(timedOut).To(BeFalse())
			Expect(realPath).To(Equal("/fake/device/path-part"))
		})
	})
})
