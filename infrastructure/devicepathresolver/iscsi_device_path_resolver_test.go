package devicepathresolver_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	fakeopeniscsi "github.com/cloudfoundry/bosh-agent/platform/openiscsi/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("iscsiDevicePathResolver", func() {
	var (
		initiatorName string
		username      string
		target        string
		password      string

		runner       *fakesys.FakeCmdRunner
		openiscsi    *fakeopeniscsi.FakeOpenIscsi
		fs           *fakesys.FakeFileSystem
		dirProvider  boshdirs.Provider
		diskSettings boshsettings.DiskSettings
		pathResolver DevicePathResolver
	)

	BeforeEach(func() {
		initiatorName = "iqn.2007-05.com.fake-domain:fake-username"
		username = "fake-username"
		target = "11.11.22.22"
		password = "fake-password"

		runner = fakesys.NewFakeCmdRunner()
		openiscsi = &fakeopeniscsi.FakeOpenIscsi{}
		fs = fakesys.NewFakeFileSystem()
		dirProvider = boshdirs.NewProvider("/fake-base-dir")

		pathResolver = NewIscsiDevicePathResolver(500*time.Millisecond, runner, openiscsi, fs, dirProvider, boshlog.NewLogger(boshlog.LevelNone))
		diskSettings = boshsettings.DiskSettings{
			ISCSISettings: boshsettings.ISCSISettings{
				InitiatorName: initiatorName,
				Username:      username,
				Target:        target,
				Password:      password,
			},
		}
	})

	Describe("GetRealDevicePath", func() {
		Context("when no devices are found", func() {
			BeforeEach(func() {
				// for the pathResolver call to find devices
				runner.AddCmdResult(
					"dmsetup ls",
					fakesys.FakeCmdResult{Stdout: "No devices found"},
				)

				openiscsi.SetupReturns(nil)

				openiscsi.DiscoveryReturns(nil)

				openiscsi.LoginReturns(nil)
			})

			It("returns the real path after iSCSI restart", func() {
				// for the pathResolver call to find devices after openiscsi restart
				runner.AddCmdResult(
					"dmsetup ls",
					fakesys.FakeCmdResult{Stdout: "fake_device_path	(252:0)"},
				)

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("/dev/mapper/fake_device_path"))
				Expect(timeout).To(BeFalse())
			})

			It("returns timeout after iSCSI restart", func() {
				// for the pathResolver call to find devices after openiscsi restart
				runner.AddCmdResult(
					"dmsetup ls",
					fakesys.FakeCmdResult{Stdout: "No devices found"},
				)

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out to get real iSCSI device path"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeTrue())

			})
		})

		Context("when one device is found", func() {
			It("returns the real path after iSCSI restart", func() {

				// for the pathResolver call to find devices
				runner.AddCmdResult(
					"dmsetup ls",
					fakesys.FakeCmdResult{Stdout: `fake_device_path-part1	(252:1)
                    fake_device_path	(252:0)
					`},
				)

				diskSettings.ID = "12345678"
				fs.WriteFileString("/fake-base-dir/bosh/managed_disk_settings.json", "12345678")
				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("/dev/mapper/fake_device_path"))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when more than 2 persistent disks are attached", func() {
			It("returns an error", func() {

				// for the pathResolver call to find devices
				runner.AddCmdResult(
					"dmsetup ls",
					fakesys.FakeCmdResult{Stdout: `fake_device_path-part1	(252:1)
					fake_device_path	(252:0)
                    fake_device_path2-part1	(252:2)
					fake_device_path2	(252:3)
                    fake_device_path3-part1	(252:2)
					fake_device_path3	(252:3)
					`},
				)

				diskSettings.ID = "22345678"
				fs.WriteFileString("/fake-base-dir/bosh/managed_disk_settings.json", "12345678")

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("More than 2 persistent disks attached"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when settings is missing the required fields", func() {
			BeforeEach(func() {
				diskSettings = boshsettings.DiskSettings{}
			})

			It("returns an error when iSCSI InitiatorName is not set", func() {
				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("iSCSI InitiatorName is not set"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeFalse())
			})

			It("returns an error when iSCSI Username is not set", func() {
				diskSettings.ISCSISettings.InitiatorName = initiatorName

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("iSCSI Username is not set"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeFalse())
			})

			It("returns an error when iSCSI Password is not set", func() {
				diskSettings.ISCSISettings.InitiatorName = initiatorName
				diskSettings.ISCSISettings.Username = username

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("iSCSI Password is not set"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeFalse())
			})

			It("returns an error when iSCSI Target is not set", func() {
				diskSettings.ISCSISettings.InitiatorName = initiatorName
				diskSettings.ISCSISettings.Username = username
				diskSettings.ISCSISettings.Password = password

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("iSCSI Target is not set"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeFalse())
			})
		})

		It("returns an error if finding devices fails", func() {
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Error: errors.New("fake-cmd-error")},
			)

			path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-cmd-error"))

			Expect(path).To(Equal(""))
			Expect(timeout).To(BeFalse())
		})

		It("returns an error if reading file fails", func() {
			// for the pathResolver call to find devices
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Stdout: "No devices found"},
			)

			diskSettings.ID = "22345678"
			fs.WriteFileString("/fake-base-dir/bosh/managed_disk_settings.json", "12345678")
			fs.RegisterReadFileError("/fake-base-dir/bosh/managed_disk_settings.json", errors.New("fake-fs-error"))

			path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Reading managed_disk_settings.json"))

			Expect(path).To(Equal(""))
			Expect(timeout).To(BeFalse())
		})

		It("returns an error if setting up open-iscsi fails", func() {
			// for the pathResolver call to find devices
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Stdout: "No devices found"},
			)

			openiscsi.SetupReturns(errors.New("fake-cmd-error"))

			path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Could not setup Open-iSCSI"))

			Expect(path).To(Equal(""))
			Expect(timeout).To(BeFalse())
		})

		It("returns an error if discovering target fails", func() {
			// for the pathResolver call to find devices
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Stdout: "No devices found"},
			)

			openiscsi.SetupReturns(nil)

			openiscsi.DiscoveryReturns(errors.New("fake-cmd-error"))

			path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Could not discovery lun"))

			Expect(path).To(Equal(""))
			Expect(timeout).To(BeFalse())
		})

		It("returns an error if logging in sessions fails", func() {
			// for the pathResolver call to find devices
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Stdout: "No devices found"},
			)

			openiscsi.SetupReturns(nil)

			openiscsi.DiscoveryReturns(nil)

			openiscsi.LoginReturns(errors.New("fake-cmd-error"))

			path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Could not login all sessions"))

			Expect(path).To(Equal(""))
			Expect(timeout).To(BeFalse())
		})

		It("returns an error if finding devices fails after open-iscsi restart", func() {
			// for the pathResolver call to find devices
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Stdout: "No devices found"},
			)

			openiscsi.SetupReturns(nil)

			openiscsi.DiscoveryReturns(nil)

			openiscsi.LoginReturns(nil)

			// for the pathResolver call to find devices after openiscsi restart
			runner.AddCmdResult(
				"dmsetup ls",
				fakesys.FakeCmdResult{Error: errors.New("fake-cmd-error")},
			)

			path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("listing mapped devices"))

			Expect(path).To(Equal(""))
			Expect(timeout).To(BeFalse())
		})

	})
})
