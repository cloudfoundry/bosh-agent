package settings_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/matchers"
	"github.com/cloudfoundry/bosh-agent/platform/disk"
	. "github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("Settings", func() {
	var settings Settings
	var updateSettings UpdateSettings

	DescribeTable("TmpFSEnabled",
		func(agentTmpFsEnabled, jobdirTmpFsEnabled, expectation bool) {
			settings.Env.Bosh.Agent.Settings.TmpFS = agentTmpFsEnabled
			settings.Env.Bosh.JobDir.TmpFS = jobdirTmpFsEnabled
			Expect(settings.TmpFSEnabled()).To(Equal(expectation))
		},
		Entry("all TmpFS settings are false", false, false, false),
		Entry("only Agent.Settings.TmpFS is set", true, false, true),
		Entry("only JobDir.TmpFS is set", false, true, true),
		Entry("both Agent.Settings.TmpFS and JobDir.TmpFS are set", true, true, true),
	)

	Describe("PersistentDiskSettings", func() {
		Context("when the disk settings are hash", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": map[string]interface{}{
								"volume_id":      "fake-disk-volume-id",
								"id":             "fake-disk-device-id",
								"path":           "fake-disk-path",
								"lun":            "fake-disk-lun",
								"host_device_id": "fake-disk-host-device-id",
							},
						},
					},
				}
			})

			It("returns disk settings", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					DeviceID:     "fake-disk-device-id",
					VolumeID:     "fake-disk-volume-id",
					Path:         "fake-disk-path",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})

			Context("when disk with requested disk ID is not present", func() {
				It("returns false", func() {
					diskSettings, found := settings.PersistentDiskSettings("fake-non-existent-disk-id")
					Expect(found).To(BeFalse())
					Expect(diskSettings).To(Equal(DiskSettings{}))
				})
			})

			Context("when Env is provided", func() {
				It("gets persistent disk settings from env", func() {
					settingsJSON := `{
						"env": {
							"persistent_disk_fs": "xfs",
							"persistent_disk_mount_options": ["opt1", "opt2"],
							"persistent_disk_partitioner": "parted"
						}
					}`

					err := json.Unmarshal([]byte(settingsJSON), &settings)
					Expect(err).NotTo(HaveOccurred())
					diskSettings, _ := settings.PersistentDiskSettings("fake-disk-id")
					Expect(settings.Env.PersistentDiskFS).To(Equal(disk.FileSystemXFS))
					Expect(diskSettings).To(Equal(DiskSettings{
						ID:             "fake-disk-id",
						DeviceID:       "fake-disk-device-id",
						VolumeID:       "fake-disk-volume-id",
						Path:           "fake-disk-path",
						Lun:            "fake-disk-lun",
						HostDeviceID:   "fake-disk-host-device-id",
						FileSystemType: "xfs",
						MountOptions:   []string{"opt1", "opt2"},
						Partitioner:    "parted",
					}))
				})

				It("does not crash if env does not have a filesystem type", func() {
					settingsJSON := `{"env": {"bosh": {"password": "secret"}}}`

					err := json.Unmarshal([]byte(settingsJSON), &settings)
					Expect(err).NotTo(HaveOccurred())
					diskSettings, _ := settings.PersistentDiskSettings("fake-disk-id")
					Expect(settings.Env.PersistentDiskFS).To(Equal(disk.FileSystemDefault))
					Expect(diskSettings).To(Equal(DiskSettings{
						ID:           "fake-disk-id",
						DeviceID:     "fake-disk-device-id",
						VolumeID:     "fake-disk-volume-id",
						Path:         "fake-disk-path",
						Lun:          "fake-disk-lun",
						HostDeviceID: "fake-disk-host-device-id",
					}))
				})

				It("does not crash if env has a bad fs", func() {
					settingsJSON := `{"env": {"persistent_disk_fs": "blahblah"}}`

					err := json.Unmarshal([]byte(settingsJSON), &settings)
					Expect(err).NotTo(HaveOccurred())
					diskSettings, _ := settings.PersistentDiskSettings("fake-disk-id")
					Expect(settings.Env.PersistentDiskFS).To(Equal(disk.FileSystemType("blahblah")))
					Expect(diskSettings).To(Equal(DiskSettings{
						ID:             "fake-disk-id",
						DeviceID:       "fake-disk-device-id",
						VolumeID:       "fake-disk-volume-id",
						Path:           "fake-disk-path",
						Lun:            "fake-disk-lun",
						HostDeviceID:   "fake-disk-host-device-id",
						FileSystemType: disk.FileSystemType("blahblah"),
					}))
				})
			})
		})

		Context("when the disk settings is a string", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": "fake-disk-value",
						},
					},
				}
			})

			It("converts it to disk settings", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:       "fake-disk-id",
					VolumeID: "fake-disk-value",
					Path:     "fake-disk-value",
				}))
			})

			Context("when disk with requested disk ID is not present", func() {
				It("returns false", func() {
					diskSettings, found := settings.PersistentDiskSettings("fake-non-existent-disk-id")
					Expect(found).To(BeFalse())
					Expect(diskSettings).To(Equal(DiskSettings{}))
				})
			})
		})

		Context("when DeviceID is not provided", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": map[string]interface{}{
								"volume_id":      "fake-disk-volume-id",
								"path":           "fake-disk-path",
								"lun":            "fake-disk-lun",
								"host_device_id": "fake-disk-host-device-id",
							},
						},
					},
				}
			})

			It("does not set id", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					VolumeID:     "fake-disk-volume-id",
					Path:         "fake-disk-path",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})
		})

		Context("when volume ID is not provided", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": map[string]interface{}{
								"id":             "fake-disk-device-id",
								"path":           "fake-disk-path",
								"lun":            "fake-disk-lun",
								"host_device_id": "fake-disk-host-device-id",
							},
						},
					},
				}
			})

			It("does not set id", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					DeviceID:     "fake-disk-device-id",
					Path:         "fake-disk-path",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})
		})

		Context("when path is not provided", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": map[string]interface{}{
								"volume_id":      "fake-disk-volume-id",
								"lun":            "fake-disk-lun",
								"host_device_id": "fake-disk-host-device-id",
							},
						},
					},
				}
			})

			It("does not set path", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					VolumeID:     "fake-disk-volume-id",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})
		})

		Context("when only (lun, host_device_id) are provided", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": map[string]interface{}{
								"lun":            "fake-disk-lun",
								"host_device_id": "fake-disk-host-device-id",
							},
						},
					},
				}
			})

			It("does not set path", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})
		})

		Context("when the disk settings contain iSCSI settings", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Persistent: map[string]interface{}{
							"fake-disk-id": map[string]interface{}{
								"id": "fake-disk-device-id",
								"iscsi_settings": map[string]interface{}{
									"initiator_name": "fake-initiator-name",
									"username":       "fake-username",
									"target":         "fake-target",
									"password":       "fake-password",
								},
							},
						},
					},
				}
			})

			It("returns disk settings", func() {
				diskSettings, found := settings.PersistentDiskSettings("fake-disk-id")
				Expect(found).To(BeTrue())
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:       "fake-disk-id",
					DeviceID: "fake-disk-device-id",
					ISCSISettings: ISCSISettings{
						InitiatorName: "fake-initiator-name",
						Username:      "fake-username",
						Target:        "fake-target",
						Password:      "fake-password",
					},
				}))
			})
		})
	})

	Describe("PersistentDiskSettingsFromHints", func() {
		Context("when the disk hint is a string", func() {
			var diskHint string

			BeforeEach(func() {
				settings = Settings{}
				diskHint = "/path/to/device/hint"
			})

			It("converts it to disk settings", func() {
				diskSettings := settings.PersistentDiskSettingsFromHint("fake-disk-id", diskHint)
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:       "fake-disk-id",
					VolumeID: "/path/to/device/hint",
					Path:     "/path/to/device/hint",
				}))
			})
		})

		Context("when the disk hint is a hash", func() {
			var diskHint map[string]interface{}

			BeforeEach(func() {
				settings = Settings{}

				diskHint = map[string]interface{}{
					"volume_id":      "fake-disk-volume-id",
					"id":             "fake-disk-device-id",
					"path":           "fake-disk-path",
					"lun":            "fake-disk-lun",
					"host_device_id": "fake-disk-host-device-id",
				}
			})

			It("returns disk settings with disk hint info", func() {
				diskSettings := settings.PersistentDiskSettingsFromHint("fake-disk-id", diskHint)
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					DeviceID:     "fake-disk-device-id",
					VolumeID:     "fake-disk-volume-id",
					Path:         "fake-disk-path",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})

			Context("when settings Env is provided", func() {
				It("gets file system type and mount options from env", func() {
					settingsJSON := `{"env": {"persistent_disk_fs": "xfs", "persistent_disk_mount_options": ["opt1", "opt2"]}}`

					err := json.Unmarshal([]byte(settingsJSON), &settings)
					Expect(err).NotTo(HaveOccurred())
					diskSettings := settings.PersistentDiskSettingsFromHint("fake-disk-id", diskHint)
					Expect(settings.Env.PersistentDiskFS).To(Equal(disk.FileSystemXFS))
					Expect(diskSettings).To(Equal(DiskSettings{
						ID:             "fake-disk-id",
						DeviceID:       "fake-disk-device-id",
						VolumeID:       "fake-disk-volume-id",
						Path:           "fake-disk-path",
						Lun:            "fake-disk-lun",
						HostDeviceID:   "fake-disk-host-device-id",
						FileSystemType: disk.FileSystemXFS,
						MountOptions:   []string{"opt1", "opt2"},
					}))
				})

				It("does not crash if env does not have a filesystem type or a persistent_disk_mount_options", func() {
					settingsJSON := `{"env": {"bosh": {"password": "secret"}}}`

					err := json.Unmarshal([]byte(settingsJSON), &settings)
					Expect(err).NotTo(HaveOccurred())
					diskSettings := settings.PersistentDiskSettingsFromHint("fake-disk-id", diskHint)
					Expect(settings.Env.PersistentDiskFS).To(Equal(disk.FileSystemDefault))
					Expect(diskSettings).To(Equal(DiskSettings{
						ID:             "fake-disk-id",
						DeviceID:       "fake-disk-device-id",
						VolumeID:       "fake-disk-volume-id",
						Path:           "fake-disk-path",
						Lun:            "fake-disk-lun",
						HostDeviceID:   "fake-disk-host-device-id",
						FileSystemType: "",
						MountOptions:   nil,
					}))
				})

				It("does not crash if env has a bad fs", func() {
					settingsJSON := `{"env": {"persistent_disk_fs": "blahblah"}}`

					err := json.Unmarshal([]byte(settingsJSON), &settings)
					Expect(err).NotTo(HaveOccurred())
					diskSettings := settings.PersistentDiskSettingsFromHint("fake-disk-id", diskHint)
					Expect(settings.Env.PersistentDiskFS).To(Equal(disk.FileSystemType("blahblah")))
					Expect(diskSettings).To(Equal(DiskSettings{
						ID:             "fake-disk-id",
						DeviceID:       "fake-disk-device-id",
						VolumeID:       "fake-disk-volume-id",
						Path:           "fake-disk-path",
						Lun:            "fake-disk-lun",
						HostDeviceID:   "fake-disk-host-device-id",
						FileSystemType: "blahblah",
						MountOptions:   nil,
					}))
				})
			})
		})

		Context("when the disk settings is nil", func() {
			BeforeEach(func() {
				settings = Settings{}
			})

			It("does NOT set device related properties in the disk settings", func() {
				diskSettings := settings.PersistentDiskSettingsFromHint("fake-disk-id", nil)
				Expect(diskSettings).To(Equal(DiskSettings{
					ID:           "fake-disk-id",
					VolumeID:     "",
					Path:         "",
					DeviceID:     "",
					Lun:          "",
					HostDeviceID: "",
				}))
			})
		})
	})

	Describe("EphemeralDiskSettings", func() {
		Context("when the disk settings are a string", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Ephemeral: "fake-disk-value",
					},
				}
			})

			It("converts disk settings", func() {
				Expect(settings.EphemeralDiskSettings()).To(Equal(DiskSettings{
					VolumeID: "fake-disk-value",
					Path:     "fake-disk-value",
				}))
			})
		})

		Context("when the disk settings are a hash", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Ephemeral: map[string]interface{}{
							"id":             "fake-disk-device-id",
							"volume_id":      "fake-disk-volume-id",
							"path":           "fake-disk-path",
							"lun":            "fake-disk-lun",
							"host_device_id": "fake-disk-host-device-id",
						},
					},
				}
			})

			It("converts disk settings", func() {
				Expect(settings.EphemeralDiskSettings()).To(Equal(DiskSettings{
					DeviceID:     "fake-disk-device-id",
					VolumeID:     "fake-disk-volume-id",
					Path:         "fake-disk-path",
					Lun:          "fake-disk-lun",
					HostDeviceID: "fake-disk-host-device-id",
				}))
			})
		})

		Context("when the disk settings are an invalid hash", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Ephemeral: []interface{}{
							"fake-disk-device-id",
							"fake-disk-volume-id",
							"fake-disk-path",
							"fake-disk-lun",
							"fake-disk-host-device-id",
						},
					},
				}
			})

			It("does not crash converting values and disk setting properties are empty", func() {
				Expect(settings.EphemeralDiskSettings()).To(Equal(DiskSettings{
					DeviceID:     "",
					VolumeID:     "",
					Path:         "",
					Lun:          "",
					HostDeviceID: "",
				}))
			})
		})

		Context("when path is not provided", func() {
			BeforeEach(func() {
				settings = Settings{
					Disks: Disks{
						Ephemeral: map[string]interface{}{
							"id":        "fake-disk-device-id",
							"volume_id": "fake-disk-volume-id",
						},
					},
				}
			})

			It("does not set path", func() {
				Expect(settings.EphemeralDiskSettings()).To(Equal(DiskSettings{
					DeviceID: "fake-disk-device-id",
					VolumeID: "fake-disk-volume-id",
				}))
			})
		})
	})

	Describe("DefaultNetworkFor", func() {
		Context("when networks is empty", func() {
			It("returns found=false", func() {
				networks := Networks{}
				_, found := networks.DefaultNetworkFor("dns")
				Expect(found).To(BeFalse())
			})
		})

		Context("with a single network", func() {
			It("returns that network (found=true)", func() {
				networks := Networks{
					"first": Network{
						DNS: []string{"xx.xx.xx.xx"},
					},
				}

				network, found := networks.DefaultNetworkFor("dns")
				Expect(found).To(BeTrue())
				Expect(network).To(Equal(networks["first"]))
			})
		})

		Context("with multiple networks and default is found for dns", func() {
			It("returns the network marked default (found=true)", func() {
				networks := Networks{
					"first": Network{
						Default: []string{},
						DNS:     []string{"aa.aa.aa.aa"},
					},
					"second": Network{
						Default: []string{"something-else", "dns"},
						DNS:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy", "zz.zz.zz.zz"},
					},
					"third": Network{
						Default: []string{},
						DNS:     []string{"aa.aa.aa.aa"},
					},
				}

				settings, found := networks.DefaultNetworkFor("dns")
				Expect(found).To(BeTrue())
				Expect(settings).To(Equal(networks["second"]))
			})
		})

		Context("with multiple networks and default is not found", func() {
			It("returns found=false", func() {
				networks := Networks{
					"first": Network{
						Default: []string{"foo"},
						DNS:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy", "zz.zz.zz.zz"},
					},
					"second": Network{
						Default: []string{},
						DNS:     []string{"aa.aa.aa.aa"},
					},
				}

				_, found := networks.DefaultNetworkFor("dns")
				Expect(found).To(BeFalse())
			})
		})

		Context("with multiple networks marked as default", func() {
			It("returns one of them", func() {
				networks := Networks{
					"first": Network{
						Default: []string{"dns"},
						DNS:     []string{"xx.xx.xx.xx", "yy.yy.yy.yy", "zz.zz.zz.zz"},
					},
					"second": Network{
						Default: []string{"dns"},
						DNS:     []string{"aa.aa.aa.aa"},
					},
					"third": Network{
						DNS: []string{"bb.bb.bb.bb"},
					},
				}

				for i := 0; i < 100; i++ {
					settings, found := networks.DefaultNetworkFor("dns")
					Expect(found).To(BeTrue())
					Expect(settings).Should(MatchOneOf(networks["first"], networks["second"]))
				}
			})
		})
	})

	Describe("DefaultIP", func() {
		It("with two networks", func() {
			networks := Networks{
				"bosh": Network{
					IP: "xx.xx.xx.xx",
				},
				"vip": Network{
					IP: "aa.aa.aa.aa",
				},
			}

			ip, found := networks.DefaultIP()
			Expect(found).To(BeTrue())
			Expect(ip).To(MatchOneOf("xx.xx.xx.xx", "aa.aa.aa.aa"))
		})

		It("with two networks only with defaults", func() {
			networks := Networks{
				"bosh": Network{
					IP: "xx.xx.xx.xx",
				},
				"vip": Network{
					IP:      "aa.aa.aa.aa",
					Default: []string{"dns"},
				},
			}

			ip, found := networks.DefaultIP()
			Expect(found).To(BeTrue())
			Expect(ip).To(Equal("aa.aa.aa.aa"))
		})

		It("when none specified", func() {
			networks := Networks{
				"bosh": Network{},
				"vip": Network{
					Default: []string{"dns"},
				},
			}

			_, found := networks.DefaultIP()
			Expect(found).To(BeFalse())
		})
	})

	It("allows different types for blobstore option values", func() {
		settingsJSON := `{"blobstore":{"options":{"string":"value", "int":443, "bool":true, "map":{}}}}`

		err := json.Unmarshal([]byte(settingsJSON), &settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Blobstore.Options).To(Equal(map[string]interface{}{
			"string": "value",
			"int":    443.0,
			"bool":   true,
			"map":    map[string]interface{}{},
		}))
	})

	Describe("Snake Case Settings", func() {
		var expectSnakeCaseKeys func(map[string]interface{})

		expectSnakeCaseKeys = func(value map[string]interface{}) {
			for k, v := range value {
				Expect(k).To(MatchRegexp("\\A[a-z0-9_]+\\z"))

				tv, isMap := v.(map[string]interface{})
				if isMap {
					expectSnakeCaseKeys(tv)
				}
			}
		}

		It("marshals into JSON in snake case to stay consistent with CPI agent env formatting", func() {
			settingsJSON, err := json.Marshal(settings)
			Expect(err).NotTo(HaveOccurred())

			var settingsMap map[string]interface{}
			err = json.Unmarshal(settingsJSON, &settingsMap)
			Expect(err).NotTo(HaveOccurred())
			expectSnakeCaseKeys(settingsMap)
		})
	})

	Describe("Network", func() {
		var network Network
		BeforeEach(func() {
			network = Network{}
		})

		Describe("IsDHCP", func() {
			Context("when network is VIP", func() {
				BeforeEach(func() {
					network.Type = NetworkTypeVIP
				})

				It("returns false", func() {
					Expect(network.IsDHCP()).To(BeFalse())
				})
			})

			Context("when network is Dynamic", func() {
				BeforeEach(func() {
					network.Type = NetworkTypeDynamic
				})

				It("returns true", func() {
					Expect(network.IsDHCP()).To(BeTrue())
				})
			})

			Context("when IP is not set", func() {
				BeforeEach(func() {
					network.Netmask = "255.255.255.0"
				})

				It("returns true", func() {
					Expect(network.IsDHCP()).To(BeTrue())
				})
			})

			Context("when Netmask is not set", func() {
				BeforeEach(func() {
					network.IP = "127.0.0.5"
				})

				It("returns true", func() {
					Expect(network.IsDHCP()).To(BeTrue())
				})
			})

			Context("when IP and Netmask are set", func() {
				BeforeEach(func() {
					network.IP = "127.0.0.5"
					network.Netmask = "255.255.255.0"
				})

				It("returns false", func() {
					Expect(network.IsDHCP()).To(BeFalse())
				})
			})

			Context("when network was previously resolved via DHCP", func() {
				BeforeEach(func() {
					network.Resolved = true
				})

				It("returns true", func() {
					Expect(network.IsDHCP()).To(BeTrue())
				})
			})

			Context("when UseDHCP is true", func() {
				BeforeEach(func() {
					network.UseDHCP = true
					network.IP = "127.0.0.5"
					network.Netmask = "255.255.255.0"
				})

				It("returns true", func() {
					Expect(network.IsDHCP()).To(BeTrue())
				})
			})
		})
	})

	Describe("Networks", func() {
		network1 := Network{}
		network2 := Network{}
		network3 := Network{}
		networks := Networks{}

		BeforeEach(func() {
			network1.Type = NetworkTypeVIP
			network2.Preconfigured = true
			network3.Preconfigured = false
		})

		Describe("IsPreconfigured", func() {
			Context("with VIP and all preconfigured networks", func() {
				BeforeEach(func() {
					networks = Networks{
						"first":  network1,
						"second": network2,
					}
				})

				It("returns true", func() {
					Expect(networks.IsPreconfigured()).To(BeTrue())
				})
			})

			Context("with VIP and NOT all preconfigured networks", func() {
				BeforeEach(func() {
					networks = Networks{
						"first":  network1,
						"second": network2,
						"third":  network3,
					}
				})

				It("returns false", func() {
					Expect(networks.IsPreconfigured()).To(BeFalse())
				})
			})

			Context("with NO VIP and all preconfigured networks", func() {
				BeforeEach(func() {
					networks = Networks{
						"first": network2,
					}
				})

				It("returns true", func() {
					Expect(networks.IsPreconfigured()).To(BeTrue())
				})
			})

			Context("with NO VIP and NOT all preconfigured networks", func() {
				BeforeEach(func() {
					networks = Networks{
						"first":  network2,
						"second": network3,
					}
				})

				It("returns false", func() {
					Expect(networks.IsPreconfigured()).To(BeFalse())
				})
			})
		})
	})

	Describe("Env", func() {
		It("unmarshal env value correctly", func() {
			var env Env
			envJSON := `{
  "bosh": {
    "password": "fake-password",
    "keep_root_password": false,
    "remove_dev_tools": true,
    "authorized_keys": [
      "fake-key"
    ],
    "swap_size": 2048,
    "parallel": 10,
	"blobstores": [
		{
			"options": {
				"bucket_name": "george",
				"encryption_key": "optional encryption key",
				"access_key_id": "optional access key id",
				"secret_access_key": "optional secret access key",
				"port": 443
			},
			"provider": "s3"
		},
		{
			"options": {
				"blobstore_path": "/var/vcap/micro_bosh/data/cache"
			},
			"provider": "local"
		}
	]
  }
}`
			err := json.Unmarshal([]byte(envJSON), &env)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.GetPassword()).To(Equal("fake-password"))
			Expect(env.GetKeepRootPassword()).To(BeFalse())
			Expect(env.GetRemoveDevTools()).To(BeTrue())
			Expect(env.Bosh.IPv6).To(Equal(IPv6{}))
			Expect(env.GetAuthorizedKeys()).To(ConsistOf("fake-key"))
			Expect(*env.GetSwapSizeInBytes()).To(Equal(uint64(2048 * 1024 * 1024)))
			Expect(*env.GetParallel()).To(Equal(10))
			Expect(env.Bosh.Blobstores).To(Equal(
				[](Blobstore){
					Blobstore{
						Type: "s3",
						Options: map[string]interface{}{
							"bucket_name":       "george",
							"encryption_key":    "optional encryption key",
							"access_key_id":     "optional access key id",
							"secret_access_key": "optional secret access key",
							"port":              443.0,
						},
					},
					Blobstore{
						Type: "local",
						Options: map[string]interface{}{
							"blobstore_path": "/var/vcap/micro_bosh/data/cache",
						},
					},
				}))
		})

		It("permits you to specify bootstrap https certs", func() {
			var env Env
			envJSON := `{
  "bosh": {
    "password": "fake-password",
    "keep_root_password": false,
    "remove_dev_tools": true,
    "authorized_keys": [
      "fake-key"
    ],
    "swap_size": 2048,
    "mbus": {
			"cert": {
				"private_key": "fake-private-key-pem",
				"certificate": "fake-certificate-pem"
      }
    }
  }
}`
			err := json.Unmarshal([]byte(envJSON), &env)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.GetPassword()).To(Equal("fake-password"))
			Expect(env.GetKeepRootPassword()).To(BeFalse())
			Expect(env.GetRemoveDevTools()).To(BeTrue())
			Expect(env.GetAuthorizedKeys()).To(ConsistOf("fake-key"))
			Expect(env.Bosh.Mbus.Cert.PrivateKey).To(Equal("fake-private-key-pem"))
			Expect(env.Bosh.Mbus.Cert.Certificate).To(Equal("fake-certificate-pem"))
			Expect(*env.GetSwapSizeInBytes()).To(Equal(uint64(2048 * 1024 * 1024)))
			Expect(*env.GetSwapSizeInBytes()).To(Equal(uint64(2048 * 1024 * 1024)))
			Expect(*env.GetSwapSizeInBytes()).To(Equal(uint64(2048 * 1024 * 1024)))
		})

		It("can enable ipv6", func() {
			env := Env{}
			err := json.Unmarshal([]byte(`{"bosh": {} }`), &env)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Bosh.IPv6).To(Equal(IPv6{}))

			env = Env{}
			err = json.Unmarshal([]byte(`{"bosh": {"ipv6": {"enable": true} } }`), &env)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Bosh.IPv6).To(Equal(IPv6{Enable: true}))
		})

		It("can enable job directory on tmpfs", func() {
			env := Env{}
			err := json.Unmarshal([]byte(`{"bosh": {} }`), &env)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Bosh.JobDir).To(Equal(JobDir{}))

			env = Env{}
			err = json.Unmarshal([]byte(`{"bosh": {"job_dir": {"tmpfs": true, "tmpfs_size": "37m"} } }`), &env)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Bosh.JobDir).To(Equal(JobDir{TmpFS: true, TmpFSSize: "37m"}))
		})

		Context("when swap_size is not specified in the json", func() {
			It("unmarshalls correctly", func() {
				var env Env
				envJSON := `{"bosh": {"password": "fake-password", "keep_root_password": false, "remove_dev_tools": true, "authorized_keys": ["fake-key"]}}`

				err := json.Unmarshal([]byte(envJSON), &env)
				Expect(err).NotTo(HaveOccurred())

				Expect(env.GetSwapSizeInBytes()).To(BeNil())
			})
		})

		Context("when parallel is not specified in the json", func() {
			It("sets to the default value", func() {
				var env Env
				envJSON := `{"bosh": {"password": "fake-password", "keep_root_password": false, "remove_dev_tools": true, "authorized_keys": ["fake-key"]}}`

				err := json.Unmarshal([]byte(envJSON), &env)
				Expect(err).NotTo(HaveOccurred())

				Expect(*env.GetParallel()).To(Equal(5))
			})
		})

		Context("#GetBlobstore", func() {
			blobstoreLocal := Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/micro_bosh/data/cache",
				},
			}

			blobstoreS3 := Blobstore{
				Type: "s3",
				Options: map[string]interface{}{
					"bucket_name":       "george",
					"encryption_key":    "optional encryption key",
					"access_key_id":     "optional access key id",
					"secret_access_key": "optional secret access key",
					"port":              443.0,
				},
			}

			blobstoreGcs := Blobstore{
				Type: "gcs",
				Options: map[string]interface{}{
					"provider": "gcs",
					"json_key": "|" +
						"DIRECTOR-BLOBSTORE-SERVICE-ACCOUNT-FILE",
					"bucket_name":    "test-bosh-bucket",
					"encryption_key": "BASE64-ENCODED-32-BYTES",
					"storage_class":  "REGIONAL",
				},
			}

			DescribeTable("agent returning the right blobstore configuration",
				func(settingsBlobstore Blobstore, envBoshBlobstores [](Blobstore), expectedBlobstore Blobstore) {
					settings := Settings{
						Blobstore: settingsBlobstore,
						Env: Env{
							Bosh: BoshEnv{
								Blobstores: envBoshBlobstores,
							},
						},
					}

					Expect(settings.GetBlobstore()).To(Equal(expectedBlobstore))
				},

				Entry("setting.Blobstore provided and env.bosh.Blobstores is missing",
					blobstoreLocal,
					nil,
					blobstoreLocal),

				Entry("setting.Blobstore is missing and env.bosh.Blobstores is provided with a single entry",
					nil,
					[]Blobstore{blobstoreLocal},
					blobstoreLocal),

				Entry("setting.Blobstore is present and env.bosh.Blobstores is provided with a single entry",
					blobstoreGcs,
					[]Blobstore{blobstoreLocal},
					blobstoreLocal),

				Entry("setting.Blobstore is missing and env.bosh.Blobstores has multiple entries",
					nil,
					[]Blobstore{blobstoreS3, blobstoreGcs},
					blobstoreS3),

				Entry("setting.Blobstore and env.bosh.Blobstores both are missing",
					nil,
					nil,
					nil),
			)
		})

		Context("#GetNtpServers", func() {
			ntpSetOne := []string{"a", "b", "c"}

			ntpSetTwo := []string{"d", "e", "f"}

			DescribeTable("agent returning the right ntp configuration",
				func(settingsNtp []string, envBoshNtp []string, expectedNtpServers []string) {
					settings := Settings{
						NTP: settingsNtp,
						Env: Env{
							Bosh: BoshEnv{
								NTP: envBoshNtp,
							},
						},
					}

					Expect(settings.GetNtpServers()).To(Equal(expectedNtpServers))
				},

				Entry("setting.ntp provided and env.bosh.ntp is missing",
					ntpSetOne,
					nil,
					ntpSetOne),

				Entry("setting.ntp is missing and env.bosh.ntp is present",
					nil,
					ntpSetTwo,
					ntpSetTwo),

				Entry("setting.ntp is present and env.bosh.ntp is present",
					ntpSetOne,
					ntpSetTwo,
					ntpSetTwo),

				Entry("setting.ntp and env.bosh.ntp both are missing",
					nil,
					nil,
					nil),
			)
		})

		Context("#IsNATSMutualTLSEnabled", func() {
			Context("env JSON does NOT provide mbus", func() {
				It("should return false", func() {
					envJSON := `{ "bosh": {} }`

					var env Env
					err := json.Unmarshal([]byte(envJSON), &env)
					Expect(err).NotTo(HaveOccurred())
					Expect(env.IsNATSMutualTLSEnabled()).To(BeFalse())
				})
			})

			DescribeTable("env JSON provides mbus",
				func(cert string, expected bool) {
					envJSON := `{ "bosh": { "mbus": { "cert": ` + cert + ` } } }`

					var env Env
					err := json.Unmarshal([]byte(envJSON), &env)
					Expect(err).NotTo(HaveOccurred())
					Expect(env.IsNATSMutualTLSEnabled()).To(Equal(expected))
				},

				Entry("empty cert",
					`{}`,
					false),

				Entry("only certificate provided",
					`{ "certificate": "some value" }`,
					false),

				Entry("only private_key provided",
					`{ "private_key": "some value" }`,
					false),

				Entry("provides certificate, and private_key",
					`{ "certificate": "some value", "private_key": "some value" }`,
					true),
			)
		})
	})

	Describe("UpdateSettings", func() {
		var updateSettingsJSON string
		BeforeEach(func() {
			updateSettingsJSON = `{"trusted_certs": "some_cert", "disk_associations": [{"name": "some_name", "cid": "some_cid"}]}`
		})
		It("contains the correct keys", func() {
			err := json.Unmarshal([]byte(updateSettingsJSON), &updateSettings)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#GetMbusURL", func() {
		Context("Env.Bosh.Mbus.URLs is populated", func() {
			It("should return Env.Bosh.Mbus.URLs", func() {
				settings = Settings{
					Env: Env{
						Bosh: BoshEnv{
							Mbus: MBus{
								URLs: []string{"nats://nested:789"},
							},
						},
					},
					Mbus: "nats://top-level:123",
				}

				Expect(settings.GetMbusURL()).To(Equal("nats://nested:789"))
			})
		})

		Context("Settings.Env.Bosh.Mbus.URLs is nil", func() {
			It("should return Settings.Mbus", func() {
				settings = Settings{
					Mbus: "nats://top-level:456",
					Env: Env{
						Bosh: BoshEnv{
							Mbus: MBus{
								URLs: nil,
							},
						},
					},
				}

				Expect(settings.GetMbusURL()).To(Equal("nats://top-level:456"))
			})
		})

		Context("Settings.Env.Bosh.Mbus.URLs is zero length", func() {
			It("should return Settings.Mbus", func() {
				settings = Settings{
					Mbus: "nats://top-level:456",
					Env: Env{
						Bosh: BoshEnv{
							Mbus: MBus{
								URLs: []string{},
							},
						},
					},
				}

				Expect(settings.GetMbusURL()).To(Equal("nats://top-level:456"))
			})
		})
	})

	Describe("HasInterfaceAlias", func() {
		Context("when networks is empty", func() {
			It("returns found=false", func() {
				networks := Networks{}
				found := networks.HasInterfaceAlias()
				Expect(found).To(BeFalse())
			})
		})

		Context("with a single network", func() {
			It("returns found=true", func() {
				networks := Networks{
					"first": Network{
						Type:  "dynamic",
						Alias: "fake-alias",
					},
				}

				found := networks.HasInterfaceAlias()
				Expect(found).To(BeTrue())
			})
		})

		Context("with multiple networks", func() {
			It("returns found=true if one of networks is set", func() {
				networks := Networks{
					"first": Network{
						Type: "dynamic",
					},
					"second": Network{
						Type: "dynamic",
					},
					"third": Network{
						Type:  "dynamic",
						Alias: "fake-alias",
					},
				}

				found := networks.HasInterfaceAlias()
				Expect(found).To(BeTrue())
			})

			It("returns found=false if the network is vip", func() {
				networks := Networks{
					"first": Network{
						Type:  "vip",
						Alias: "fake-alias",
					},
					"second": Network{
						Type: "dynamic",
					},
				}

				found := networks.HasInterfaceAlias()
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("NetmaskToCIDR", func() {
		Context("ipv6", func() {
			It("converts valid netmasks", func() {
				cidr, err := NetmaskToCIDR("ffff:ffff:ffff:ffff::", true)
				Expect(cidr).To(Equal("64"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("errors when a netmask is unconvertible", func() {
				_, err := NetmaskToCIDR("ffff:ffff:0000:ffff::", true)
				Expect(err).To(HaveOccurred())
			})

			It("properly converts zero netmasks", func() {
				cidr, err := NetmaskToCIDR("::", true)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).To(Equal("0"))
			})
		})

		Context("ipv4", func() {
			It("converts valid netmasks", func() {
				cidr, err := NetmaskToCIDR("255.255.0.0", false)
				Expect(cidr).To(Equal("16"))
				Expect(err).NotTo(HaveOccurred())
			})
			It("errors when a netmask is unconvertible", func() {
				_, err := NetmaskToCIDR("255.0.255.0", false)
				Expect(err).To(HaveOccurred())
			})

			It("properly converts zero netmasks", func() {
				cidr, err := NetmaskToCIDR("0.0.0.0", false)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).To(Equal("0"))
			})
		})
	})
})
