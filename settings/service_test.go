package settings_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	. "github.com/cloudfoundry/bosh-agent/settings"
	"github.com/cloudfoundry/bosh-agent/settings/settingsfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("settingsService", func() {
	var (
		fs                         *fakesys.FakeFileSystem
		fakePlatformSettingsGetter *settingsfakes.FakePlatformSettingsGetter
		fakeSettingsSource         *fakes.FakeSettingsSource
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		fakePlatformSettingsGetter = &settingsfakes.FakePlatformSettingsGetter{}
		fakePlatformSettingsGetter.GetAgentSettingsPathReturns("/setting/path.json")
		fakePlatformSettingsGetter.GetPersistentDiskSettingsPathReturns("/setting/persistent_settings.json")
		fakeSettingsSource = &fakes.FakeSettingsSource{}
	})

	buildService := func() (Service, *fakesys.FakeFileSystem) {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		service := NewService(
			fs,
			fakeSettingsSource,
			fakePlatformSettingsGetter,
			logger,
		)
		return service, fs
	}

	Describe("LoadSettings", func() {
		var (
			fetchedSettings Settings
			fetcherFuncErr  error
			service         Service
		)

		BeforeEach(func() {
			fetchedSettings = Settings{}
			fetcherFuncErr = nil
		})

		JustBeforeEach(func() {
			fakeSettingsSource.SettingsValue = fetchedSettings
			fakeSettingsSource.SettingsErr = fetcherFuncErr
			service, fs = buildService()
		})

		Context("when settings fetcher succeeds fetching settings", func() {
			BeforeEach(func() {
				fetchedSettings = Settings{AgentID: "some-new-agent-id"}
			})

			Context("thread safe", func() {
				BeforeEach(func() {
					fetchedSettings.Networks = Networks{
						"fake-net-1": Network{Type: NetworkTypeDynamic},
					}
				})

				It("should ensure only one thread at a time is writing or reading the settings", func() {
					done := make(chan bool)

					go func() {
						for i := 0; i < 100000; i++ {
							service.GetSettings()
						}
						done <- true
					}()

					go func() {
						for i := 0; i < 100000; i++ {
							service.LoadSettings()
						}
						done <- true
					}()

					for i := 0; i < 2; i++ {
						<-done
					}
				})
			})

			Context("when logging settings.json write information", func() {
				It("should remain quiet about the contents of the settings.json in the log", func() {
					err := service.LoadSettings()
					Expect(err).NotTo(HaveOccurred())

					Expect(fs.WriteFileQuietlyCallCount).To(Equal(1))
					Expect(fs.WriteFileCallCount).To(Equal(0))
				})
			})

			Context("when settings contain at most one dynamic network", func() {
				It("updates the service with settings from the fetcher", func() {
					err := service.LoadSettings()
					Expect(err).NotTo(HaveOccurred())
					Expect(service.GetSettings().AgentID).To(Equal("some-new-agent-id"))
				})

				It("persists settings to the settings file", func() {
					err := service.LoadSettings()
					Expect(err).NotTo(HaveOccurred())

					json, err := json.Marshal(fetchedSettings)
					Expect(err).NotTo(HaveOccurred())

					fileContent, err := fs.ReadFile("/setting/path.json")
					Expect(err).NotTo(HaveOccurred())
					Expect(fileContent).To(Equal(json))
				})

				It("returns any error from writing to the setting file", func() {
					fs.WriteFileError = errors.New("fs-write-file-error")

					err := service.LoadSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fs-write-file-error"))
				})
			})
		})

		Context("when tmpfs is disabled", func() {
			It("does not call SetupBoshSettingsDisk()", func() {
				err := service.LoadSettings()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePlatformSettingsGetter.SetupBoshSettingsDiskCallCount()).To(Equal(0))
			})
		})

		Context("when tmpfs is enabled", func() {
			BeforeEach(func() {
				fetchedSettings.Env.Bosh.Agent.Settings.TmpFS = true
			})

			It("calls SetupBoshSettingsDisk before writing settings", func() {
				err := service.LoadSettings()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePlatformSettingsGetter.SetupBoshSettingsDiskCallCount()).To(Equal(1))
			})
		})

		Context("when settings fetcher fails fetching settings", func() {
			BeforeEach(func() {
				fetcherFuncErr = errors.New("fake-fetch-error")
			})

			Context("when a settings file exists", func() {
				Context("when settings contain at most one dynamic network", func() {
					BeforeEach(func() {
						fs.WriteFile("/setting/path.json", []byte(`{
								"agent_id":"some-agent-id",
								"networks": {"fake-net-1": {"type": "dynamic"}}
							}`))

						fakePlatformSettingsGetter.GetDefaultNetworkReturns(Network{
							IP:      "fake-resolved-ip",
							Netmask: "fake-resolved-netmask",
							Gateway: "fake-resolved-gateway",
						}, nil)
					})

					It("should remain quiet about the contents of the settings.json in the log", func() {
						err := service.LoadSettings()
						Expect(err).NotTo(HaveOccurred())

						Expect(fs.ReadFileWithOptsCallCount).To(Equal(1))
					})

					It("returns settings from the settings file with resolved network", func() {
						err := service.LoadSettings()
						Expect(err).ToNot(HaveOccurred())
						Expect(service.GetSettings()).To(Equal(Settings{
							AgentID: "some-agent-id",
							Networks: Networks{
								"fake-net-1": Network{
									Type:     NetworkTypeDynamic,
									IP:       "fake-resolved-ip",
									Netmask:  "fake-resolved-netmask",
									Gateway:  "fake-resolved-gateway",
									Resolved: true,
								},
							},
						}))
					})
				})
			})

			Context("when non-unmarshallable settings file exists", func() {
				It("returns any error from the fetcher", func() {
					fs.WriteFile("/setting/path.json", []byte(`$%^&*(`))

					err := service.LoadSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-fetch-error"))

					Expect(service.GetSettings()).To(Equal(Settings{}))
				})
			})

			Context("when no settings file exists", func() {
				It("returns any error from the fetcher", func() {
					err := service.LoadSettings()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-fetch-error"))

					Expect(service.GetSettings()).To(Equal(Settings{}))
				})
			})
		})
	})

	Describe("GetPersistentDiskSettings", func() {
		var (
			fetchedSettings Settings
			fetcherFuncErr  error
			service         Service
		)

		BeforeEach(func() {
			fetchedSettings = Settings{}
			fetcherFuncErr = nil
		})

		JustBeforeEach(func() {
			fakeSettingsSource.SettingsValue = fetchedSettings
			fakeSettingsSource.SettingsErr = fetcherFuncErr
			service, fs = buildService()

			err := service.LoadSettings()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("disk settings file does not exist on disk", func() {
			It("returns and error", func() {
				_, err := service.GetPersistentDiskSettings("fake-disk-cid")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("disk settings file exists", func() {
			BeforeEach(func() {
				err := fs.WriteFileQuietly("/setting/persistent_settings.json", []byte("{}"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("loads persistent settings from disk", func() {
				service.GetPersistentDiskSettings("fake-disk-cid")
				Expect(fs.ReadFileWithOptsCallCount).To(Equal(1))
			})

			Context("has invalid settings saved on disk", func() {
				var existingSettingsOnDisk []DiskSettings // The correct format is map[string]DiskSettings but we want to write out an incorrect format.

				BeforeEach(func() {
					service, fs = buildService()
					existingSettingsOnDisk = []DiskSettings{
						{ID: "1", Path: "abc"},
						{ID: "2", Path: "def"},
						{ID: "3", Path: "ghi"},
					}
					jsonString, err := json.Marshal(existingSettingsOnDisk)
					Expect(err).NotTo(HaveOccurred())

					err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns error", func() {
					_, err := service.GetPersistentDiskSettings("1")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Getting all persistent disk settings: Reading persistent disk settings: Unmarshalling persistent disk settings from file"))
				})
			})

			Context("has valid settings saved on disk", func() {
				var expectedDiskSettings DiskSettings

				BeforeEach(func() {
					service, fs = buildService()
					writeSettings := map[string]DiskSettings{
						"1": {
							ID:   "1",
							Path: "abc",
						},
					}
					expectedDiskSettings = writeSettings["1"]
					jsonString, err := json.Marshal(writeSettings)
					Expect(err).NotTo(HaveOccurred())

					err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns all disk settings", func() {
					diskSettings, err := service.GetPersistentDiskSettings("1")
					Expect(err).ToNot(HaveOccurred())
					Expect(diskSettings).To(Equal(expectedDiskSettings))
				})
			})

			Context("settings only present in agent settings file", func() {
				BeforeEach(func() {
					fetchedSettings = Settings{
						Disks: Disks{
							Persistent: map[string]interface{}{
								"disk-cid": "/dev/sda",
							},
						},
					}
				})
				It("returns disk settings", func() {
					settings, err := service.GetPersistentDiskSettings("disk-cid")
					Expect(err).ToNot(HaveOccurred())
					Expect(settings).To(Equal(DiskSettings{
						ID:       "disk-cid",
						Path:     "/dev/sda",
						VolumeID: "/dev/sda",
					}))
				})
			})
		})
	})

	Describe("GetAllPersistentDiskSettings", func() {
		var service Service

		BeforeEach(func() {
			service, fs = buildService()

			existingSettingsOnDisk := map[string]DiskSettings{
				"1": {ID: "1", Path: "abc"},
			}
			jsonString, err := json.Marshal(existingSettingsOnDisk)
			Expect(err).NotTo(HaveOccurred())

			err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
			Expect(err).ToNot(HaveOccurred())

			fakeSettingsSource.SettingsValue = Settings{
				Disks: Disks{
					Persistent: map[string]interface{}{
						"2": "xyz",
					},
				},
			}
			err = service.LoadSettings()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("both locations for disk settings", func() {
			It("combines all disk settings", func() {
				allSettings, err := service.GetAllPersistentDiskSettings()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(allSettings)).To(Equal(2))
				Expect(allSettings["1"]).ToNot(BeNil())
				Expect(allSettings["2"]).ToNot(BeNil())
			})
		})
	})

	Describe("SavePersistentDiskSettings", func() {
		var (
			service                Service
			existingSettingsOnDisk map[string]DiskSettings
		)

		BeforeEach(func() {
			service, fs = buildService()
		})

		Context("persistent disk settings file does not exist", func() {
			It("creates persistent disk settings on disk using provided hint", func() {
				diskSettings := DiskSettings{ID: "2", DeviceID: "def"}
				existingSettingsOnDisk = make(map[string]DiskSettings)
				existingSettingsOnDisk["2"] = diskSettings

				err := service.SavePersistentDiskSettings(diskSettings)
				Expect(err).NotTo(HaveOccurred())

				jsonString, err := json.Marshal(existingSettingsOnDisk)
				Expect(err).NotTo(HaveOccurred())

				fileContent, err := fs.ReadFile("/setting/persistent_settings.json")
				Expect(err).NotTo(HaveOccurred())
				Expect(fileContent).To(Equal(jsonString))
			})
		})

		Context("persistent disk settings file exists", func() {
			Context("has invalid settings saved on disk", func() {
				var existingSettingsOnDisk []DiskSettings // The correct format is map[string]DiskSettings but we want to write out an incorrect format.

				BeforeEach(func() {
					service, fs = buildService()
					existingSettingsOnDisk = []DiskSettings{
						{ID: "1", Path: "abc"},
						{ID: "2", Path: "def"},
						{ID: "3", Path: "ghi"},
					}
					jsonString, err := json.Marshal(existingSettingsOnDisk)
					Expect(err).NotTo(HaveOccurred())

					err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns error", func() {
					diskSettings := DiskSettings{ID: "2", DeviceID: "def"}
					err := service.SavePersistentDiskSettings(diskSettings)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Unmarshalling persistent disk settings from file"))
				})
			})

			Context("has valid settings saved on disk", func() {
				var existingSettingsOnDisk map[string]DiskSettings

				BeforeEach(func() {
					existingSettingsOnDisk = map[string]DiskSettings{"1": {ID: "1", Path: "abc"}}
					jsonString, err := json.Marshal(existingSettingsOnDisk)
					Expect(err).NotTo(HaveOccurred())

					err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
					Expect(err).ToNot(HaveOccurred())
				})

				It("reads and updates persistent disk settings on disk with provided hint", func() {
					diskSettings := DiskSettings{ID: "2", DeviceID: "def"}
					err := service.SavePersistentDiskSettings(diskSettings)
					Expect(err).NotTo(HaveOccurred())

					existingSettingsOnDisk["2"] = diskSettings
					jsonString, err := json.Marshal(existingSettingsOnDisk)
					Expect(err).NotTo(HaveOccurred())

					fileContent, err := fs.ReadFile("/setting/persistent_settings.json")
					Expect(err).NotTo(HaveOccurred())
					Expect(fileContent).To(Equal(jsonString))
				})
			})
		})
	})

	Describe("RemovePersistentDiskSettings", func() {
		var (
			service Service
		)

		BeforeEach(func() {
			service, fs = buildService()
		})

		Context("persistent disk settings file does not exist", func() {
			It("completes without error", func() {
				err := service.RemovePersistentDiskSettings("anything")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("persistent disk settings file exists", func() {
			Context("has invalid settings saved on disk", func() {
				var existingSettingsOnDisk []DiskSettings // The correct format is map[string]DiskSettings but we want to write out an incorrect format.

				BeforeEach(func() {
					service, fs = buildService()
					existingSettingsOnDisk = []DiskSettings{
						{ID: "1", Path: "abc"},
						{ID: "2", Path: "def"},
						{ID: "3", Path: "ghi"},
					}
					jsonString, err := json.Marshal(existingSettingsOnDisk)
					Expect(err).NotTo(HaveOccurred())

					err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns error", func() {
					err := service.RemovePersistentDiskSettings("1")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Cannot remove entry from file due to read error"))
					Expect(err.Error()).To(ContainSubstring("Unmarshalling persistent disk settings from file"))
				})
			})

			Context("has valid settings saved on disk", func() {

				Context("file has single entry", func() {
					var existingSettingsOnDisk map[string]DiskSettings

					BeforeEach(func() {
						existingSettingsOnDisk = map[string]DiskSettings{"1": {ID: "1", Path: "abc"}}
						jsonString, err := json.Marshal(existingSettingsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
						Expect(err).ToNot(HaveOccurred())
					})

					It("does not do anything if id does not exist in file and does not return an error", func() {
						err := service.RemovePersistentDiskSettings("anything")
						Expect(err).NotTo(HaveOccurred())

						jsonString, err := json.Marshal(existingSettingsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						fileContent, err := fs.ReadFile("/setting/persistent_settings.json")
						Expect(err).NotTo(HaveOccurred())
						Expect(fileContent).To(Equal(jsonString))
					})

					It("the file is still valid after the last entry is deleted", func() {
						err := service.RemovePersistentDiskSettings("1")
						Expect(err).NotTo(HaveOccurred())

						err = service.RemovePersistentDiskSettings("anything")
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Context("file has multiple entries", func() {
					var existingSettingsOnDisk map[string]DiskSettings

					BeforeEach(func() {
						existingSettingsOnDisk = map[string]DiskSettings{
							"1": {ID: "1", Path: "abc"},
							"2": {ID: "2", Path: "abc"},
							"3": {ID: "3", Path: "abc"},
						}
						jsonString, err := json.Marshal(existingSettingsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						err = fs.WriteFileQuietly("/setting/persistent_settings.json", jsonString)
						Expect(err).ToNot(HaveOccurred())
					})

					It("deletes the hint if it exists in the file", func() {
						err := service.RemovePersistentDiskSettings("1")
						Expect(err).NotTo(HaveOccurred())

						delete(existingSettingsOnDisk, "1")
						jsonString, err := json.Marshal(existingSettingsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						fileContent, err := fs.ReadFile("/setting/persistent_settings.json")
						Expect(err).NotTo(HaveOccurred())
						Expect(fileContent).To(Equal(jsonString))
					})
				})
			})
		})
	})

	Describe("InvalidateSettings", func() {
		It("removes the settings file", func() {
			fakeSettingsSource.SettingsValue = Settings{}
			fakeSettingsSource.SettingsErr = nil
			service, fs := buildService()

			fs.WriteFile("/setting/path.json", []byte(`{}`))

			err := service.InvalidateSettings()
			Expect(err).ToNot(HaveOccurred())

			Expect(fs.FileExists("/setting/path.json")).To(BeFalse())
		})

		It("returns err if removing settings file errored", func() {
			fakeSettingsSource.SettingsValue = Settings{}
			fakeSettingsSource.SettingsErr = nil
			service, fs := buildService()

			fs.RemoveAllStub = func(_ string) error {
				return errors.New("fs-remove-all-error")
			}

			err := service.InvalidateSettings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fs-remove-all-error"))
		})
	})

	Describe("GetSettings", func() {
		var (
			loadedSettings Settings
			service        Service
		)

		BeforeEach(func() {
			loadedSettings = Settings{AgentID: "some-agent-id"}
		})

		JustBeforeEach(func() {
			fakeSettingsSource.SettingsValue = loadedSettings
			fakeSettingsSource.SettingsErr = nil
			service, _ = buildService()
			err := service.LoadSettings()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is are no dynamic networks", func() {
			It("returns settings without modifying any networks", func() {
				Expect(service.GetSettings()).To(Equal(loadedSettings))
			})

			It("does not try to determine default network", func() {
				_ = service.GetSettings()
				Expect(fakePlatformSettingsGetter.GetDefaultNetworkCallCount()).To(Equal(0))
			})
		})

		Context("when there is network that needs to be resolved (ip, netmask, or mac are not set)", func() {
			BeforeEach(func() {
				loadedSettings = Settings{
					Networks: map[string]Network{
						"fake-net1": Network{
							IP:      "fake-net1-ip",
							Netmask: "fake-net1-netmask",
							Mac:     "fake-net1-mac",
							Gateway: "fake-net1-gateway",
						},
						"fake-net2": Network{
							Gateway: "fake-net2-gateway",
							DNS:     []string{"fake-net2-dns"},
						},
					},
				}
			})

			Context("when default network can be retrieved", func() {
				BeforeEach(func() {
					fakePlatformSettingsGetter.GetDefaultNetworkReturns(Network{
						IP:      "fake-resolved-ip",
						Netmask: "fake-resolved-netmask",
						Gateway: "fake-resolved-gateway",
					}, nil)
				})

				It("returns settings with resolved dynamic network ip, netmask, gateway and keeping everything else the same", func() {
					settings := service.GetSettings()
					Expect(settings).To(Equal(Settings{
						Networks: map[string]Network{
							"fake-net1": Network{
								IP:      "fake-net1-ip",
								Netmask: "fake-net1-netmask",
								Mac:     "fake-net1-mac",
								Gateway: "fake-net1-gateway",
							},
							"fake-net2": Network{
								IP:       "fake-resolved-ip",
								Netmask:  "fake-resolved-netmask",
								Gateway:  "fake-resolved-gateway",
								DNS:      []string{"fake-net2-dns"},
								Resolved: true,
							},
						},
					}))
				})
			})

			Context("when default network fails to be retrieved", func() {
				BeforeEach(func() {
					fakePlatformSettingsGetter.GetDefaultNetworkReturns(Network{}, errors.New("fake-get-default-network-err"))
				})

				It("returns error", func() {
					settings := service.GetSettings()
					Expect(settings).To(Equal(loadedSettings))
				})
			})
		})
	})
})
