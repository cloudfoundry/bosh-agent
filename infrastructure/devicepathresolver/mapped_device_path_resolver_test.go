package devicepathresolver_test

import (
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"errors"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("mappedDevicePathResolver", func() {
	var (
		fs           *fakesys.FakeFileSystem
		diskSettings boshsettings.DiskSettings
		resolver     DevicePathResolver
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("Not yet implemented on Windows")
		}

		fs = fakesys.NewFakeFileSystem()
		resolver = NewMappedDevicePathResolver(time.Second, fs)
		diskSettings = boshsettings.DiskSettings{
			Path: "/dev/sda",
		}
	})

	Context("when path is not provided", func() {
		It("returns an error", func() {
			diskSettings := boshsettings.DiskSettings{}
			_, _, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path is missing"))
		})
	})

	Context("when a matching /dev/xvdX device is found", func() {
		BeforeEach(func() {
			err := fs.WriteFile("/dev/xvda", []byte{})
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFile("/dev/vda", []byte{})
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFile("/dev/sda", []byte{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the match", func() {
			realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(timedOut).To(BeFalse())
			Expect(realPath).To(Equal("/dev/xvda"))
		})
	})

	Context("when a matching /dev/vdX device is found", func() {
		BeforeEach(func() {
			err := fs.WriteFile("/dev/vda", []byte{})
			Expect(err).NotTo(HaveOccurred())
			err = fs.WriteFile("/dev/sda", []byte{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the match", func() {
			realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(timedOut).To(BeFalse())
			Expect(realPath).To(Equal("/dev/vda"))
		})
	})

	Context("when a matching /dev/sdX device is found", func() {
		BeforeEach(func() {
			err := fs.WriteFile("/dev/sda", []byte{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the match", func() {
			realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(timedOut).To(BeFalse())
			Expect(realPath).To(Equal("/dev/sda"))
		})
	})

	Context("when a matching /dev/nvmeXn1 device is found", func() {
		BeforeEach(func() {
			err := fs.WriteFile("/dev/nvme0n1", []byte{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the match", func() {
			realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
			Expect(err).NotTo(HaveOccurred())
			Expect(timedOut).To(BeFalse())
			Expect(realPath).To(Equal("/dev/nvme0n1"))
		})
	})

	Context("when no matching device is found the first time", func() {
		Context("when the timeout has not expired", func() {
			BeforeEach(func() {
				time.AfterFunc(time.Second, func() {
					err := fs.WriteFile("/dev/xvda", []byte{})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			It("returns the match", func() {
				realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
				Expect(err).NotTo(HaveOccurred())
				Expect(timedOut).To(BeFalse())
				Expect(realPath).To(Equal("/dev/xvda"))
			})
		})

		Context("when the timeout has expired", func() {
			It("errs", func() {
				_, timedOut, err := resolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Timed out getting real device path for /dev/sda"))
				Expect(timedOut).To(BeTrue())
			})
		})
	})

	Context("when a path that never needs remapping is passed in", func() {
		Context("when path exists", func() {
			BeforeEach(func() {
				err := fs.WriteFile("/dev/xvdba", []byte{})
				Expect(err).NotTo(HaveOccurred())
				diskSettings = boshsettings.DiskSettings{
					Path: "/dev/xvdba",
				}
			})

			It("returns the path as given", func() {
				realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
				Expect(realPath).To(Equal("/dev/xvdba"))
				Expect(err).ToNot(HaveOccurred())
				Expect(timedOut).To(BeFalse())
			})
		})

		Context("when path is a symlink", func() {
			BeforeEach(func() {
				Expect(fs.WriteFile("/dev/xvdba", []byte{})).To(Succeed())
				Expect(fs.MkdirAll("/dev/disk/by-label", 0755)).To(Succeed())

				diskSettings = boshsettings.DiskSettings{
					Path: "/dev/disk/by-label/my-disk-label",
				}
			})

			Context("and resolves to absolute path", func() {
				BeforeEach(func() {
					Expect(fs.Symlink("/dev/xvdba", "/dev/disk/by-label/my-disk-label")).To(Succeed())
				})

				It("returns the resolved path", func() {
					realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
					Expect(realPath).To(Equal("/dev/xvdba"))
					Expect(err).ToNot(HaveOccurred())
					Expect(timedOut).To(BeFalse())
				})
			})

			Context("and resolves to a relative path", func() {
				BeforeEach(func() {
					Expect(fs.Symlink("../../xvdba", "/dev/disk/by-label/my-disk-label")).To(Succeed())
				})

				It("returns the resolved path", func() {
					realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
					Expect(realPath).To(Equal("/dev/xvdba"))
					Expect(err).ToNot(HaveOccurred())
					Expect(timedOut).To(BeFalse())
				})
			})

			Context("and ReadLink fails", func() {
				BeforeEach(func() {
					Expect(fs.Symlink("/dev/xvdba", "/dev/disk/by-label/my-disk-label")).To(Succeed())
					fs.ReadlinkError = errors.New("can't read symlink")
				})

				It("returns error", func() {
					_, timedOut, err := resolver.GetRealDevicePath(diskSettings)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Getting real device path: can't read symlink"))
					Expect(timedOut).To(BeFalse())
				})
			})
		})

		Context("when path does not exist", func() {
			BeforeEach(func() {
				diskSettings = boshsettings.DiskSettings{
					Path: "/dev/xvdba",
				}
			})

			It("returns an error", func() {
				realPath, timedOut, err := resolver.GetRealDevicePath(diskSettings)
				Expect(realPath).To(Equal(""))
				Expect(err).To(HaveOccurred())
				Expect(timedOut).To(BeTrue())
			})
		})
	})
})
