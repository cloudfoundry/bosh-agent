package action_test

import (
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"

	"errors"
	"github.com/cloudfoundry/bosh-agent/platform/disk"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
)

var _ = Describe("UnmountDiskAction", func() {
	var (
		platform *platformfakes.FakePlatform
		action   UnmountDiskAction

		expectedDiskSettings boshsettings.DiskSettings
		settingsService      *fakesettings.FakeSettingsService
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}

		settingsService = &fakesettings.FakeSettingsService{
			PersistentDiskSettings: map[string]boshsettings.DiskSettings{
				"vol-123": {
					ID:           "vol-123",
					VolumeID:     "2",
					Path:         "/dev/sdf",
					Lun:          "0",
					HostDeviceID: "fake-host-device-id",
					ISCSISettings: boshsettings.ISCSISettings{
						InitiatorName: "fake-initiator-name",
						Username:      "fake-username",
						Password:      "fake-password",
						Target:        "fake-target",
					},
					FileSystemType: disk.FileSystemExt4,
				},
			},
		}

		action = NewUnmountDisk(settingsService, platform)

		expectedDiskSettings = boshsettings.DiskSettings{
			ID:             "vol-123",
			VolumeID:       "2",
			Path:           "/dev/sdf",
			FileSystemType: "ext4",
			Lun:            "0",
			HostDeviceID:   "fake-host-device-id",
			ISCSISettings: boshsettings.ISCSISettings{
				InitiatorName: "fake-initiator-name",
				Username:      "fake-username",
				Password:      "fake-password",
				Target:        "fake-target",
			},
		}
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	It("unmount disk when the disk is mounted", func() {
		platform.UnmountPersistentDiskReturns(true, nil)

		result, err := action.Run("vol-123")
		Expect(err).ToNot(HaveOccurred())
		boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Unmounted partition of {ID:vol-123 DeviceID: VolumeID:2 Lun:0 HostDeviceID:fake-host-device-id Path:/dev/sdf ISCSISettings:{InitiatorName:fake-initiator-name Username:fake-username Target:fake-target Password:fake-password} FileSystemType:ext4 MountOptions:[] Partitioner:}"}`)

		Expect(platform.UnmountPersistentDiskCallCount()).To(Equal(1))
		Expect(platform.UnmountPersistentDiskArgsForCall(0)).To(Equal(expectedDiskSettings))
	})

	It("unmount disk when the disk is not mounted", func() {
		platform.UnmountPersistentDiskReturns(false, nil)

		result, err := action.Run("vol-123")
		Expect(err).ToNot(HaveOccurred())
		boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Partition of {ID:vol-123 DeviceID: VolumeID:2 Lun:0 HostDeviceID:fake-host-device-id Path:/dev/sdf ISCSISettings:{InitiatorName:fake-initiator-name Username:fake-username Target:fake-target Password:fake-password} FileSystemType:ext4 MountOptions:[] Partitioner:} is not mounted"}`)

		Expect(platform.UnmountPersistentDiskCallCount()).To(Equal(1))
		Expect(platform.UnmountPersistentDiskArgsForCall(0)).To(Equal(expectedDiskSettings))
	})

	Context("error getting persistent disk settings", func() {
		BeforeEach(func() {
			settingsService.GetPersistentDiskSettingsError = errors.New("DNE")
		})

		It("returns error", func() {
			_, err := action.Run("vol-456")
			Expect(err).To(HaveOccurred())
		})
	})

})
