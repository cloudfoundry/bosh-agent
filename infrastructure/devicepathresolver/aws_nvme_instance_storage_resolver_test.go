package devicepathresolver_test

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	fakedpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("AWSNVMeInstanceStorageResolver", func() {
	var (
		resolver               InstanceStorageResolver
		fakeFS                 *fakesys.FakeFileSystem
		fakeDevicePathResolver *fakedpresolv.FakeDevicePathResolver
		logger                 boshlog.Logger
	)
	BeforeEach(func() {
		fakeFS = fakesys.NewFakeFileSystem()
		fakeDevicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		resolver = NewAWSNVMeInstanceStorageResolver(fakeFS, fakeDevicePathResolver, logger, "", "")
	})
	Describe("DiscoverInstanceStorage", func() {
		Context("when devices are NVMe", func() {
			It("discovers instance storage by filtering out EBS volumes", func() {
				devices := []boshsettings.DiskSettings{
					{Path: "/dev/nvme0n1"},
					{Path: "/dev/nvme1n1"},
				}

				// Create device files
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
					return nil, nil
				}

				err = fakeFS.Symlink("/dev/nvme0n1", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-root")
				Expect(err).NotTo(HaveOccurred())

				paths, err := resolver.DiscoverInstanceStorage(devices)
				Expect(err).NotTo(HaveOccurred())
				Expect(paths).To(Equal([]string{"/dev/nvme1n1", "/dev/nvme2n1"}))
			})
			It("returns error if not enough instance storage devices found", func() {
				devices := []boshsettings.DiskSettings{
					{Path: "/dev/nvme0n1"},
					{Path: "/dev/nvme1n1"},
					{Path: "/dev/nvme2n1"},
				}

				// Create device files
				err := fakeFS.WriteFileString("/dev/nvme0n1", "")
				Expect(err).NotTo(HaveOccurred())
				err = fakeFS.WriteFileString("/dev/nvme1n1", "")
				Expect(err).NotTo(HaveOccurred())

				fakeFS.GlobStub = func(pattern string) ([]string, error) {
					if pattern == "/dev/nvme*n1" {
						return []string{"/dev/nvme0n1", "/dev/nvme1n1"}, nil
					}
					if pattern == "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*" {
						return []string{"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-root"}, nil
					}
					return nil, nil
				}

				err = fakeFS.Symlink("/dev/nvme0n1", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-root")
				Expect(err).NotTo(HaveOccurred())

				_, err = resolver.DiscoverInstanceStorage(devices)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Expected 3 instance storage devices but discovered 1"))
			})
			It("returns error if too many instance storage devices found", func() {
				devices := []boshsettings.DiskSettings{
					{Path: "/dev/nvme0n1"},
					{Path: "/dev/nvme1n1"},
				}

				// Create device files
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
					// No EBS symlinks - all devices are instance storage
					return nil, nil
				}

				// No symlinks to filter out - all 3 devices will be returned as instance storage
				_, err = resolver.DiscoverInstanceStorage(devices)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Expected 2 instance storage devices but discovered 3"))
			})
		})
	})
})
