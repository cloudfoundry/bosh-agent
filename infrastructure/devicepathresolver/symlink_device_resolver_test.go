package devicepathresolver_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
)

var _ = Describe("SymlinkDeviceResolver", func() {
	var (
		fs       *fakesys.FakeFileSystem
		logger   boshlog.Logger
		resolver *SymlinkDeviceResolver
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		resolver = NewSymlinkDeviceResolver(fs, logger)
	})

	Describe("ResolveSymlinksToDevices", func() {
		It("returns empty map when no symlinks match the pattern", func() {
			fs.SetGlob("/dev/disk/by-id/nvme-*", []string{})

			result, err := resolver.ResolveSymlinksToDevices("/dev/disk/by-id/nvme-*")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("resolves symlinks to their target device paths", func() {
			err := fs.MkdirAll("/dev/disk/by-id", os.FileMode(0750))
			Expect(err).ToNot(HaveOccurred())

			// Create target device files
			err = fs.WriteFileString("/dev/nvme1n1", "")
			Expect(err).ToNot(HaveOccurred())
			err = fs.WriteFileString("/dev/nvme2n1", "")
			Expect(err).ToNot(HaveOccurred())

			fs.SetGlob("/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*", []string{
				"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol123",
				"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol456",
			})
			err = fs.Symlink("/dev/nvme1n1", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol123")
			Expect(err).ToNot(HaveOccurred())
			err = fs.Symlink("/dev/nvme2n1", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol456")
			Expect(err).ToNot(HaveOccurred())

			result, err := resolver.ResolveSymlinksToDevices("/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_*")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result["/dev/nvme1n1"]).To(Equal("/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol123"))
			Expect(result["/dev/nvme2n1"]).To(Equal("/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol456"))
		})

		It("skips symlinks that cannot be resolved", func() {
			err := fs.MkdirAll("/dev/disk/by-id", os.FileMode(0750))
			Expect(err).ToNot(HaveOccurred())

			// Create target device file for valid symlink
			err = fs.WriteFileString("/dev/nvme1n1", "")
			Expect(err).ToNot(HaveOccurred())

			fs.SetGlob("/dev/disk/by-id/nvme-*", []string{
				"/dev/disk/by-id/nvme-valid",
				"/dev/disk/by-id/nvme-invalid",
			})
			err = fs.Symlink("/dev/nvme1n1", "/dev/disk/by-id/nvme-valid")
			Expect(err).ToNot(HaveOccurred())
			// nvme-invalid has no symlink target

			result, err := resolver.ResolveSymlinksToDevices("/dev/disk/by-id/nvme-*")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result["/dev/nvme1n1"]).To(Equal("/dev/disk/by-id/nvme-valid"))
		})

		It("returns error when glob fails", func() {
			fs.GlobErr = errors.New("glob error")

			_, err := resolver.ResolveSymlinksToDevices("/dev/disk/by-id/nvme-*")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("glob error"))
		})
	})

	Describe("GetDevicesByPattern", func() {
		It("returns devices matching the pattern", func() {
			fs.SetGlob("/dev/nvme*n1", []string{"/dev/nvme0n1", "/dev/nvme1n1", "/dev/nvme2n1"})

			devices, err := resolver.GetDevicesByPattern("/dev/nvme*n1")
			Expect(err).ToNot(HaveOccurred())
			Expect(devices).To(ConsistOf("/dev/nvme0n1", "/dev/nvme1n1", "/dev/nvme2n1"))
		})

		It("returns empty slice when no devices match", func() {
			fs.SetGlob("/dev/nvme*n1", []string{})

			devices, err := resolver.GetDevicesByPattern("/dev/nvme*n1")
			Expect(err).ToNot(HaveOccurred())
			Expect(devices).To(BeEmpty())
		})

		It("returns error when glob fails", func() {
			fs.GlobErr = errors.New("glob error")

			_, err := resolver.GetDevicesByPattern("/dev/nvme*n1")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("FilterDevices", func() {
		It("returns devices not in the exclusion map", func() {
			allDevices := []string{"/dev/nvme0n1", "/dev/nvme1n1", "/dev/nvme2n1", "/dev/nvme3n1"}
			excludeDevices := map[string]string{
				"/dev/nvme1n1": "/dev/disk/by-id/ebs-vol1",
				"/dev/nvme2n1": "/dev/disk/by-id/ebs-vol2",
			}

			filtered := resolver.FilterDevices(allDevices, excludeDevices)
			Expect(filtered).To(ConsistOf("/dev/nvme0n1", "/dev/nvme3n1"))
		})

		It("returns all devices when exclusion map is empty", func() {
			allDevices := []string{"/dev/nvme0n1", "/dev/nvme1n1"}
			excludeDevices := map[string]string{}

			filtered := resolver.FilterDevices(allDevices, excludeDevices)
			Expect(filtered).To(ConsistOf("/dev/nvme0n1", "/dev/nvme1n1"))
		})

		It("returns empty slice when all devices are excluded", func() {
			allDevices := []string{"/dev/nvme0n1"}
			excludeDevices := map[string]string{
				"/dev/nvme0n1": "/dev/disk/by-id/ebs-vol1",
			}

			filtered := resolver.FilterDevices(allDevices, excludeDevices)
			Expect(filtered).To(BeEmpty())
		})
	})
})
