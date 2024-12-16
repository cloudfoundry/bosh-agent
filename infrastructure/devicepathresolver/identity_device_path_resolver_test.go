package devicepathresolver_test

import (
	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IdentityDevicePathResolver", func() {
	var (
		identityDevicePathResolver DevicePathResolver
	)

	BeforeEach(func() {
		identityDevicePathResolver = NewIdentityDevicePathResolver()
	})

	Context("when path is not provided", func() {
		It("returns an error", func() {
			diskSettings := boshsettings.DiskSettings{}
			_, _, err := identityDevicePathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path is missing"))
		})
	})
})
