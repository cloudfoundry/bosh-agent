package devicepathresolver_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("IdentityInstanceStorageResolver", func() {
	var (
		resolver               InstanceStorageResolver
		fakeDevicePathResolver *fakedpresolv.FakeDevicePathResolver
	)

	BeforeEach(func() {
		fakeDevicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		resolver = NewIdentityInstanceStorageResolver(fakeDevicePathResolver)
	})

	Describe("DiscoverInstanceStorage", func() {
		It("returns device paths resolved by the underlying device path resolver", func() {
			devices := []boshsettings.DiskSettings{
				{Path: "/dev/xvdb"},
				{Path: "/dev/xvdc"},
			}

			fakeDevicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}

			paths, err := resolver.DiscoverInstanceStorage(devices)
			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(Equal([]string{"/dev/xvdb", "/dev/xvdc"}))
			Expect(fakeDevicePathResolver.GetRealDevicePathCallCount()).To(Equal(2))
		})

		It("returns error if device path resolver fails", func() {
			devices := []boshsettings.DiskSettings{
				{Path: "/dev/xvdb"},
			}

			fakeDevicePathResolver.GetRealDevicePathReturns("", false, errors.New("fake-error"))

			_, err := resolver.DiscoverInstanceStorage(devices)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-error"))
		})

		It("returns empty slice for empty input", func() {
			devices := []boshsettings.DiskSettings{}

			paths, err := resolver.DiscoverInstanceStorage(devices)
			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(Equal([]string{}))
		})
	})
})
