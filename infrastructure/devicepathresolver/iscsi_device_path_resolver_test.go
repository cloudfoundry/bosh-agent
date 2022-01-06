package devicepathresolver_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	"github.com/cloudfoundry/bosh-agent/platform/openiscsi/openiscsifakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

type dmsetupDevice struct {
	Name      string
	MajMinNum string
}

func buildDmsetupOutput(devices []dmsetupDevice) (output string) {
	for _, device := range devices {
		output += fmt.Sprintf("%s\t%s\n", device.Name, device.MajMinNum)
	}
	return
}

var _ = Describe("iscsiDevicePathResolver", func() {
	var (
		initiatorName string
		username      string
		target        string
		password      string

		runner                  *fakesys.FakeCmdRunner
		openiscsi               *openiscsifakes.FakeOpenIscsi
		fs                      *fakesys.FakeFileSystem
		dirProvider             boshdirs.Provider
		diskSettings            boshsettings.DiskSettings
		pathResolver            DevicePathResolver
		managedDiskSettingsPath string

		dmsetupOutputForNoDeviceFound = fakesys.FakeCmdResult{
			Stdout: "No devices found"}
		dmsetupOutputForNonPartitionedDisk = fakesys.FakeCmdResult{
			Stdout: buildDmsetupOutput(
				[]dmsetupDevice{
					{Name: "fake_device_path", MajMinNum: "(252:0)"},
				}),
		}
		dmsetupOutputForPartitionedDisk = fakesys.FakeCmdResult{Stdout: buildDmsetupOutput(
			[]dmsetupDevice{
				{Name: "fake_device_path-part1", MajMinNum: "(252:1)"},
				{Name: "fake_device_path", MajMinNum: "(252:0)"},
			}),
		}
	)

	BeforeEach(func() {
		initiatorName = "iqn.2007-05.com.fake-domain:fake-username"
		username = "fake-username"
		target = "11.11.22.22"
		password = "fake-password"

		runner = fakesys.NewFakeCmdRunner()
		openiscsi = &openiscsifakes.FakeOpenIscsi{}
		fs = fakesys.NewFakeFileSystem()
		dirProvider = boshdirs.NewProvider("/fake-base-dir")

		pathResolver = NewIscsiDevicePathResolver(
			500*time.Millisecond,
			runner,
			openiscsi,
			fs,
			dirProvider,
			boshlog.NewLogger(boshlog.LevelNone),
		)
		diskSettings = boshsettings.DiskSettings{
			ISCSISettings: boshsettings.ISCSISettings{
				InitiatorName: initiatorName,
				Username:      username,
				Target:        target,
				Password:      password,
			},
		}

		// Setup the managed_disk_settings.json
		managedDiskSettingsPath = filepath.Join(dirProvider.BoshDir(), "managed_disk_settings.json")
		err := fs.WriteFileString(managedDiskSettingsPath, "12345678")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetRealDevicePath", func() {
		Context("when no devices are found", func() {
			BeforeEach(func() {
				// for the pathResolver call to find devices
				runner.AddCmdResult(
					"dmsetup ls",
					dmsetupOutputForNoDeviceFound)

				openiscsi.SetupReturns(nil)

				openiscsi.DiscoveryReturns(nil)

				openiscsi.LoginReturns(nil)
			})

			It("returns the real path after iSCSI restart", func() {
				// for the pathResolver call to find devices after openiscsi restart
				runner.AddCmdResult(
					"dmsetup ls",
					dmsetupOutputForNonPartitionedDisk)

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("/dev/mapper/fake_device_path"))
				Expect(timeout).To(BeFalse())

				Expect(openiscsi.SetupCallCount()).To(Equal(1))
				Expect(openiscsi.DiscoveryCallCount()).To(Equal(1))
				Expect(openiscsi.LoginCallCount()).To(Equal(1))
			})

			It("returns timeout after iSCSI restart", func() {
				// for the pathResolver call to find devices after openiscsi restart
				runner.AddCmdResult(
					"dmsetup ls",
					dmsetupOutputForNoDeviceFound)

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Timed out to get real iSCSI device path"))

				Expect(path).To(Equal(""))
				Expect(timeout).To(BeTrue())
			})
		})

		Context("when one device is to be found", func() {
			BeforeEach(func() {
				diskSettings.ID = "12345678"
			})

			Context("when the device has never been mounted previously", func() {
				BeforeEach(func() {
					// ...that has never been mounted previously...
					fs.RemoveAll(managedDiskSettingsPath)
				})

				Context("when the device is not yet partitioned", func() {
					BeforeEach(func() {
						runner.AddCmdResult("dmsetup ls",
							dmsetupOutputForNoDeviceFound)
						runner.AddCmdResult("dmsetup ls",
							dmsetupOutputForNonPartitionedDisk)
					})

					It("returns the real path after iSCSI discovery", func() {
						path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
						Expect(err).ToNot(HaveOccurred())

						Expect(path).To(Equal("/dev/mapper/fake_device_path"))
						Expect(timeout).To(BeFalse())

						Expect(openiscsi.SetupCallCount()).To(Equal(1))
						Expect(openiscsi.DiscoveryCallCount()).To(Equal(1))
						Expect(openiscsi.LogoutCallCount()).To(Equal(0))
						Expect(openiscsi.LoginCallCount()).To(Equal(1))
					})

					Context("when already logged in to iSCSI", func() {
						BeforeEach(func() {
							openiscsi.IsLoggedinReturns(true, nil)
						})

						It("returns the real path after iSCSI logout/login", func() {
							path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
							Expect(err).ToNot(HaveOccurred())

							Expect(path).To(Equal("/dev/mapper/fake_device_path"))
							Expect(timeout).To(BeFalse())

							Expect(openiscsi.SetupCallCount()).To(Equal(1))
							Expect(openiscsi.DiscoveryCallCount()).To(Equal(1))
							Expect(openiscsi.LogoutCallCount()).To(Equal(1))
							Expect(openiscsi.LoginCallCount()).To(Equal(1))
						})
					})
				})

				Context("when the device is already partitioned", func() {
					BeforeEach(func() {
						runner.AddCmdResult("dmsetup ls",
							dmsetupOutputForPartitionedDisk)
					})

					It("returns the real path without iSCSI restart", func() {
						path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
						Expect(err).ToNot(HaveOccurred())

						Expect(path).To(Equal("/dev/mapper/fake_device_path"))
						Expect(timeout).To(BeFalse())

						Expect(openiscsi.SetupCallCount()).To(Equal(0))
						Expect(openiscsi.DiscoveryCallCount()).To(Equal(0))
						Expect(openiscsi.LogoutCallCount()).To(Equal(0))
						Expect(openiscsi.LoginCallCount()).To(Equal(0))
					})
				})

				Context("when device is resolved then paritioned and re-resolved", func() {
					BeforeEach(func() {
						runner.AddCmdResult("dmsetup ls",
							dmsetupOutputForNoDeviceFound)
						runner.AddCmdResult("dmsetup ls",
							dmsetupOutputForNonPartitionedDisk)
						runner.AddCmdResult("dmsetup ls",
							dmsetupOutputForPartitionedDisk)
					})

					It("returns the real path consistently", func() {
						path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
						Expect(err).ToNot(HaveOccurred())

						Expect(path).To(Equal("/dev/mapper/fake_device_path"))
						Expect(timeout).To(BeFalse())

						Expect(openiscsi.LoginCallCount()).To(Equal(1))

						path, timeout, err = pathResolver.GetRealDevicePath(diskSettings)
						Expect(err).ToNot(HaveOccurred())

						Expect(path).To(Equal("/dev/mapper/fake_device_path"))
						Expect(timeout).To(BeFalse())

						// expect no more call to Login() to have been made:
						Expect(openiscsi.LoginCallCount()).To(Equal(1))
					})
				})
			})

			Context("when the device has already been mounted previously", func() {
				BeforeEach(func() {
					runner.AddCmdResult("dmsetup ls",
						dmsetupOutputForPartitionedDisk)
				})

				It("returns the real path without iSCSI restart", func() {
					path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
					Expect(err).ToNot(HaveOccurred())

					Expect(path).To(Equal("/dev/mapper/fake_device_path"))
					Expect(timeout).To(BeFalse())

					Expect(openiscsi.SetupCallCount()).To(Equal(0))
					Expect(openiscsi.DiscoveryCallCount()).To(Equal(0))
					Expect(openiscsi.LogoutCallCount()).To(Equal(0))
					Expect(openiscsi.LoginCallCount()).To(Equal(0))
				})
			})
		})

		Context("when performing a disk migration", func() {
			BeforeEach(func() {
				// NOTE: this test setup is based on the traces provided in
				// https://github.com/cloudfoundry/bosh-agent/issues/252#issuecomment-977712657
				dmsetupOutputForPartitionedDisk = fakesys.FakeCmdResult{Stdout: buildDmsetupOutput(
					[]dmsetupDevice{
						{Name: "3600a098038305679445d523053437757", MajMinNum: "(253:0)"},
						{Name: "3600a098038305679445d523053437757-part1", MajMinNum: "(253:1)"},
					}),
				}
				runner.AddCmdResult("dmsetup ls",
					dmsetupOutputForPartitionedDisk)
				runner.AddCmdResult("dmsetup ls",
					dmsetupOutputForPartitionedDisk)
				runner.AddCmdResult("dmsetup ls",
					fakesys.FakeCmdResult{Stdout: buildDmsetupOutput(
						[]dmsetupDevice{
							{Name: "3600a098038305678762b523053333956", MajMinNum: "(253:2)"},
							{Name: "3600a098038305679445d523053437757", MajMinNum: "(253:0)"},
							{Name: "3600a098038305679445d523053437757-part1", MajMinNum: "(253:1)"},
						}),
					})
				runner.AddCmdResult("dmsetup ls",
					fakesys.FakeCmdResult{Stdout: buildDmsetupOutput(
						[]dmsetupDevice{
							{Name: "3600a098038305678762b523053333956", MajMinNum: "(253:2)"},
							{Name: "3600a098038305678762b523053333956-part1", MajMinNum: "(253:3)"},
							{Name: "3600a098038305679445d523053437757", MajMinNum: "(253:0)"},
							{Name: "3600a098038305679445d523053437757-part1", MajMinNum: "(253:1)"},
						}),
					})
			})

			It("returns the real path consistently", func() {
				diskSettings.ID = "12345678" // the last mounted disk ID

				path, timeout, err := pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("/dev/mapper/3600a098038305679445d523053437757"))
				Expect(timeout).To(BeFalse())

				diskSettings.ID = "23456789" // the new disk with new size

				path, timeout, err = pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("/dev/mapper/3600a098038305678762b523053333956"))
				Expect(timeout).To(BeFalse())

				path, timeout, err = pathResolver.GetRealDevicePath(diskSettings)
				Expect(err).ToNot(HaveOccurred())

				Expect(path).To(Equal("/dev/mapper/3600a098038305678762b523053333956"))
				Expect(timeout).To(BeFalse())
			})
		})

		Context("when more than 2 persistent disks are attached", func() {
			It("returns an error", func() {
				// for the pathResolver call to find devices
				runner.AddCmdResult(
					"dmsetup ls",
					fakesys.FakeCmdResult{Stdout: buildDmsetupOutput(
						[]dmsetupDevice{
							{Name: "fake_device_path-part1", MajMinNum: "(252:1)"},
							{Name: "fake_device_path", MajMinNum: "(252:0)"},
							{Name: "fake_device_path2-part1", MajMinNum: "(252:3)"},
							{Name: "fake_device_path2", MajMinNum: "(252:2)"},
							{Name: "fake_device_path3-part1", MajMinNum: "(252:5)"},
							{Name: "fake_device_path3", MajMinNum: "(252:4)"},
						}),
					})

				diskSettings.ID = "22345678"

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
			diskSettings.ID = "22345678"
			fs.RegisterReadFileError(managedDiskSettingsPath, errors.New("fake-fs-error"))

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
				dmsetupOutputForNoDeviceFound,
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
				dmsetupOutputForNoDeviceFound,
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
				dmsetupOutputForNoDeviceFound,
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
				dmsetupOutputForNoDeviceFound,
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
