package devicepathresolver_test

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	fakeudev "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
)

var _ = Describe("VirtioVolumeIDDevicePathResolver", func() {
	var (
		fs           *fakesys.FakeFileSystem
		udev         *fakeudev.FakeUdevDevice
		diskSettings boshsettings.DiskSettings
		pathResolver DevicePathResolver
	)

	BeforeEach(func() {
		udev = fakeudev.NewFakeUdevDevice()
		fs = fakesys.NewFakeFileSystem()
		diskSettings = boshsettings.DiskSettings{
			VolumeID: "vol-1234567890abcdef0",
		}
		pathResolver = NewVirtioVolumeIDDevicePathResolver(500*time.Millisecond, udev, fs, boshlog.NewLogger(boshlog.LevelNone))
	})

	Describe("GetRealDevicePath", func() {
		It("refreshes udev", func() {
			pathResolver.GetRealDevicePath(diskSettings)
			Expect(udev.Triggered).To(Equal(true))
			Expect(udev.Settled).To(Equal(true))
		})

		Context("when virtio device path exists", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("/dev", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.MkdirAll("/dev/vdb", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("/dev/vdb", "/dev/disk/by-id/virtio-vol-1234567890abcdef0")
				Expect(err).ToNot(HaveOccurred())

				fs.SetGlob("/dev/disk/by-id/*vol-1234567890abcdef0*", []string{"/dev/disk/by-id/virtio-vol-1234567890abcdef0"})
			})

			It("returns fully resolved the path", func() {
				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				devicePath, err := filepath.Abs("/dev/vdb")
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal(devicePath))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when nvme device path exists", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("/dev", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.MkdirAll("/dev/nvme1n1", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("/dev/nvme1n1", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-1234567890abcdef0")
				Expect(err).ToNot(HaveOccurred())

				fs.SetGlob("/dev/disk/by-id/*vol-1234567890abcdef0*", []string{"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-1234567890abcdef0"})
			})

			It("returns fully resolved the path", func() {
				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				devicePath, err := filepath.Abs("/dev/nvme1n1")
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal(devicePath))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when multiple disks with same volume ID exist", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("fake-device-path-1", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())
				err = fs.MkdirAll("fake-device-path-2", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("fake-device-path-1", "/dev/disk/by-id/virtio-vol-1234567890abcdef0")
				Expect(err).ToNot(HaveOccurred())
				err = fs.Symlink("fake-device-path-2", "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-1234567890abcdef0")
				Expect(err).ToNot(HaveOccurred())

				fs.SetGlob("/dev/disk/by-id/*vol-1234567890abcdef0*", []string{
					"/dev/disk/by-id/virtio-vol-1234567890abcdef0",
					"/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol-1234567890abcdef0",
				})
			})
			It("returns an error", func() {
				_, _, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("More than one disk matched"))
			})
		})

		Context("when path does not exist", func() {
			BeforeEach(func() {
				err := fs.Symlink("fake-device-path", "/dev/disk/by-id/virtio-vol-1234567890abcdef0")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns a timeout error", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out getting real device path for VolumeID 'vol-1234567890abcdef0'"))
				Expect(timeout).To(BeTrue())
			})
		})

		Context("when symlink does not exist", func() {
			It("returns a timeout error", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out getting real device path for VolumeID 'vol-1234567890abcdef0'"))
				Expect(timeout).To(BeTrue())
			})
		})

		Context("when no matching device is found the first time", func() {
			Context("when the timeout has not expired", func() {
				BeforeEach(func() {
					err := fs.MkdirAll("/fake-device-path", os.FileMode(0750))
					Expect(err).ToNot(HaveOccurred())

					err = fs.MkdirAll("/dev/disk/by-id", os.FileMode(0750))
					Expect(err).ToNot(HaveOccurred())

					err = fs.Symlink("/fake-device-path", "/dev/disk/by-id/virtio-vol-1234567890abcdef0")
					Expect(err).ToNot(HaveOccurred())

					fs.GlobStub = func(pattern string) ([]string, error) {
						fs.SetGlob("/dev/disk/by-id/*vol-1234567890abcdef0*", []string{
							"/dev/disk/by-id/virtio-vol-1234567890abcdef0",
						})

						fs.GlobStub = nil

						return nil, errors.New("new error")
					}
				})

				It("returns the real path", func() {
					path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
					Expect(err).ToNot(HaveOccurred())

					devicePath, err := filepath.Abs("/fake-device-path")
					Expect(err).ToNot(HaveOccurred())

					Expect(path).To(Equal(devicePath))
					Expect(timeout).To(BeFalse())
				})
			})
		})

		Context("when triggering udev fails", func() {
			BeforeEach(func() {
				udev.TriggerErr = errors.New("fake-udev-trigger-error")
			})

			It("returns an error", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-udev-trigger-error"))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when settling udev fails", func() {
			BeforeEach(func() {
				udev.SettleErr = errors.New("fake-udev-settle-error")
			})

			It("returns an error", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-udev-settle-error"))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when VolumeID is empty", func() {
			BeforeEach(func() {
				diskSettings = boshsettings.DiskSettings{}
			})

			It("returns an error", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Disk VolumeID is not set"))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when VolumeID is not the correct format", func() {
			BeforeEach(func() {
				diskSettings = boshsettings.DiskSettings{
					VolumeID: "too-short",
				}
			})

			It("returns an error", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Disk VolumeID is not the correct format"))
				Expect(timeout).To(BeFalse())
			})
		})
	})
})
