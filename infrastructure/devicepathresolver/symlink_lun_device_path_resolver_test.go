package devicepathresolver_test

import (
	"os"
	"runtime"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
)

var _ = Describe("SymlinkLunDevicePathResolver", func() {
	var (
		fs           *fakesys.FakeFileSystem
		diskSettings boshsettings.DiskSettings
		pathResolver DevicePathResolver
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("Not applicable on Windows")
		}

		fs = fakesys.NewFakeFileSystem()
		pathResolver = NewSymlinkLunDevicePathResolver(
			"/dev/disk/azure/data/by-lun",
			500*time.Millisecond,
			fs,
			boshlog.NewLogger(boshlog.LevelNone),
		)
		diskSettings = boshsettings.DiskSettings{
			Lun:          "2",
			HostDeviceID: "fake-host-device-id",
		}
	})

	Describe("GetRealDevicePath", func() {
		Context("when lun is not set", func() {
			It("returns an error", func() {
				diskSettings.Lun = ""
				_, _, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Disk lun is not set"))
			})
		})

		Context("when symlink exists and points to a real device (SCSI)", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("/dev/disk/azure/data/by-lun", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("/dev/sdc", "/dev/disk/azure/data/by-lun/2")
				Expect(err).ToNot(HaveOccurred())

				err = fs.MkdirAll("/dev/sdc", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the real device path", func() {
				realPath, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(timeout).To(BeFalse())
				Expect(realPath).To(Equal("/dev/sdc"))
			})
		})

		Context("when symlink exists and points to a real NVMe device", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("/dev/disk/azure/data/by-lun", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("/dev/nvme0n4", "/dev/disk/azure/data/by-lun/2")
				Expect(err).ToNot(HaveOccurred())

				err = fs.MkdirAll("/dev/nvme0n4", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the real NVMe device path", func() {
				realPath, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(timeout).To(BeFalse())
				Expect(realPath).To(Equal("/dev/nvme0n4"))
			})
		})

		Context("when symlink does not exist", func() {
			It("times out", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out"))
				Expect(timeout).To(BeTrue())
			})
		})

		Context("when symlink exists but real path does not exist yet", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("/dev/disk/azure/data/by-lun", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("/dev/sdc", "/dev/disk/azure/data/by-lun/2")
				Expect(err).ToNot(HaveOccurred())
				// Note: /dev/sdc is NOT created - real path does not exist yet
			})

			It("times out waiting for the real device", func() {
				_, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out"))
				Expect(timeout).To(BeTrue())
			})
		})

		Context("with LUN 0 (ephemeral disk)", func() {
			BeforeEach(func() {
				diskSettings.Lun = "0"

				err := fs.MkdirAll("/dev/disk/azure/data/by-lun", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())

				err = fs.Symlink("/dev/nvme0n2", "/dev/disk/azure/data/by-lun/0")
				Expect(err).ToNot(HaveOccurred())

				err = fs.MkdirAll("/dev/nvme0n2", os.FileMode(0750))
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the correct device for LUN 0", func() {
				realPath, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())
				Expect(timeout).To(BeFalse())
				Expect(realPath).To(Equal("/dev/nvme0n2"))
			})
		})
	})
})
