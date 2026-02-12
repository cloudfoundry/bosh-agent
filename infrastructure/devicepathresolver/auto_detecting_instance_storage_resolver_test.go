package devicepathresolver_test

import (
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("AutoDetectingInstanceStorageResolver", func() {
	var (
		resolver               InstanceStorageResolver
		fakeFS                 *fakesys.FakeFileSystem
		fakeDevicePathResolver *fakedpresolv.FakeDevicePathResolver
		logger                 boshlog.Logger
	)

	BeforeEach(func() {
		if runtime.GOOS != "linux" {
			Skip("Only supported on Linux")
		}
		fakeFS = fakesys.NewFakeFileSystem()
		fakeDevicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	Context("when CPI provides NVMe device paths", func() {
		It("automatically uses AWS NVMe instance storage discovery", func() {
			resolver = NewAutoDetectingInstanceStorageResolver(
				fakeFS,
				fakeDevicePathResolver,
				logger,
				"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*",
				"/dev/nvme*n1",
			)

			devices := []boshsettings.DiskSettings{
				{Path: "/dev/nvme0n1"},
				{Path: "/dev/nvme1n1"},
			}

			err := fakeFS.WriteFileString("/dev/nvme0n1", "")
			Expect(err).NotTo(HaveOccurred())
			err = fakeFS.WriteFileString("/dev/nvme1n1", "")
			Expect(err).NotTo(HaveOccurred())
			err = fakeFS.WriteFileString("/dev/nvme2n1", "")
			Expect(err).NotTo(HaveOccurred())

			fakeFS.GlobStub = func(pattern string) ([]string, error) {
				if pattern == "/dev/nvme*n1" {
					return []string{"/dev/nvme0n1", "/dev/nvme1n1", "/dev/nvme2n1"}, nil
				}
				if pattern == "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*" {
					return []string{"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-root"}, nil
				}
				return []string{}, nil
			}

			err = fakeFS.Symlink("/dev/nvme0n1", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-root")
			Expect(err).NotTo(HaveOccurred())

			paths, err := resolver.DiscoverInstanceStorage(devices)
			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(Equal([]string{"/dev/nvme1n1", "/dev/nvme2n1"}))
		})
	})

	Context("when CPI provides non-NVMe device paths", func() {
		It("automatically uses identity resolution", func() {
			resolver = NewAutoDetectingInstanceStorageResolver(
				fakeFS,
				fakeDevicePathResolver,
				logger,
				"",
				"",
			)

			devices := []boshsettings.DiskSettings{
				{Path: "/dev/xvdba"},
				{Path: "/dev/xvdbb"},
			}

			fakeDevicePathResolver.GetRealDevicePathStub = func(diskSettings boshsettings.DiskSettings) (string, bool, error) {
				return diskSettings.Path, false, nil
			}

			paths, err := resolver.DiscoverInstanceStorage(devices)
			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(Equal([]string{"/dev/xvdba", "/dev/xvdbb"}))
		})
	})

	Context("when device list is empty", func() {
		It("returns empty list", func() {
			resolver = NewAutoDetectingInstanceStorageResolver(
				fakeFS,
				fakeDevicePathResolver,
				logger,
				"",
				"",
			)

			paths, err := resolver.DiscoverInstanceStorage([]boshsettings.DiskSettings{})
			Expect(err).NotTo(HaveOccurred())
			Expect(paths).To(BeEmpty())
		})
	})
})
