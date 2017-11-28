package action_test

import (
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
)

var _ = Describe("UnmountDiskAction", func() {
	var (
		platform *fakeplatform.FakePlatform
		action   UnmountDiskAction

		expectedDiskSettings boshsettings.DiskSettings
	)

	BeforeEach(func() {
		platform = fakeplatform.NewFakePlatform()

		settingsService := &fakesettings.FakeSettingsService{
			Settings: boshsettings.Settings{
				Disks: boshsettings.Disks{
					Persistent: map[string]interface{}{
						"vol-123": map[string]interface{}{
							"volume_id":      "2",
							"path":           "/dev/sdf",
							"lun":            "0",
							"host_device_id": "fake-host-device-id",
							"initiator_name": "fake-initiator-name",
							"username":       "fake-username",
							"password":       "fake-password",
							"target":         "fake-target",
						},
					},
				},
				Env: boshsettings.Env{
					PersistentDiskFS: "ext4",
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
			InitiatorName:  "fake-initiator-name",
			Username:       "fake-username",
			Password:       "fake-password",
			Target:         "fake-target",
		}
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	It("unmount disk when the disk is mounted", func() {
		platform.UnmountPersistentDiskDidUnmount = true

		result, err := action.Run("vol-123")
		Expect(err).ToNot(HaveOccurred())
		boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Unmounted partition of {ID:vol-123 DeviceID: VolumeID:2 Lun:0 HostDeviceID:fake-host-device-id Path:/dev/sdf InitiatorName:fake-initiator-name Username:fake-username Target:fake-target Password:fake-password FileSystemType:ext4 MountOptions:[]}"}`)

		Expect(platform.UnmountPersistentDiskSettings).To(Equal(expectedDiskSettings))
	})

	It("unmount disk when the disk is not mounted", func() {
		platform.UnmountPersistentDiskDidUnmount = false

		result, err := action.Run("vol-123")
		Expect(err).ToNot(HaveOccurred())
		boshassert.MatchesJSONString(GinkgoT(), result, `{"message":"Partition of {ID:vol-123 DeviceID: VolumeID:2 Lun:0 HostDeviceID:fake-host-device-id Path:/dev/sdf InitiatorName:fake-initiator-name Username:fake-username Target:fake-target Password:fake-password FileSystemType:ext4 MountOptions:[]} is not mounted"}`)

		Expect(platform.UnmountPersistentDiskSettings).To(Equal(expectedDiskSettings))
	})

	It("unmount disk when device path not found", func() {
		_, err := action.Run("vol-456")
		Expect(err).To(HaveOccurred())
	})
})
