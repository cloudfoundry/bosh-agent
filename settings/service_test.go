package settings_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	fakenet "github.com/cloudfoundry/bosh-agent/platform/net/fakes"
	. "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() {
	Describe("settingsService", func() {
		var (
			fs                         *fakesys.FakeFileSystem
			fakeDefaultNetworkResolver *fakenet.FakeDefaultNetworkResolver
			fakeSettingsSource         *fakes.FakeSettingsSource
		)

		BeforeEach(func() {
			fs = fakesys.NewFakeFileSystem()
			fakeDefaultNetworkResolver = &fakenet.FakeDefaultNetworkResolver{}
			fakeSettingsSource = &fakes.FakeSettingsSource{}
		})

		buildService := func() (Service, *fakesys.FakeFileSystem) {
			logger := boshlog.NewLogger(boshlog.LevelNone)
			service := NewService(fs, "/setting/path.json", "/setting/persistent_hints.json", fakeSettingsSource, fakeDefaultNetworkResolver, logger)
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

							fakeDefaultNetworkResolver.GetDefaultNetworkNetwork = Network{
								IP:      "fake-resolved-ip",
								Netmask: "fake-resolved-netmask",
								Gateway: "fake-resolved-gateway",
							}
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

		Describe("GetPersistentDiskHints", func() {
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

			Context("disk hints file does not exist on disk", func() {
				It("returns empty map of disk hints", func() {
					diskHints, err := service.GetPersistentDiskHints()
					Expect(err).ToNot(HaveOccurred())
					Expect(diskHints).To(Equal(make(map[string]DiskSettings)))
				})
			})

			Context("disk hints file exists", func() {
				BeforeEach(func() {
					err := fs.WriteFileQuietly("/setting/persistent_hints.json", []byte("{}"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("loads persistent hints from disk", func() {
					service.GetPersistentDiskHints()
					Expect(fs.ReadFileWithOptsCallCount).To(Equal(1))
				})

				Context("has invalid hints saved on disk", func() {
					var existingHintsOnDisk []DiskSettings // The correct format is map[string]DiskSettings but we want to write out an incorrect format.

					BeforeEach(func() {
						service, fs = buildService()
						existingHintsOnDisk = []DiskSettings{
							{ID: "1", Path: "abc"},
							{ID: "2", Path: "def"},
							{ID: "3", Path: "ghi"},
						}
						jsonString, err := json.Marshal(existingHintsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						err = fs.WriteFileQuietly("/setting/persistent_hints.json", jsonString)
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns error", func() {
						_, err := service.GetPersistentDiskHints()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Unmarshalling persistent disk hints from file"))
					})
				})

				Context("has valid hints saved on disk", func() {
					var existingHintsOnDisk map[string]DiskSettings

					BeforeEach(func() {
						service, fs = buildService()
						existingHintsOnDisk = map[string]DiskSettings{
							"1": {ID: "1", Path: "abc"},
							"2": {ID: "2", Path: "def"},
							"3": {ID: "3", Path: "ghi"},
						}
						jsonString, err := json.Marshal(existingHintsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						err = fs.WriteFileQuietly("/setting/persistent_hints.json", jsonString)
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns all disk hints", func() {
						diskHints, err := service.GetPersistentDiskHints()
						Expect(err).ToNot(HaveOccurred())
						Expect(diskHints).To(Equal(existingHintsOnDisk))
					})
				})
			})
		})

		Describe("SavePersistentDiskHint", func() {
			var (
				service             Service
				existingHintsOnDisk map[string]DiskSettings
			)

			BeforeEach(func() {
				service, fs = buildService()
			})

			Context("persistent disk hints file does not exist", func() {
				It("creates persistent disk hints on disk using provided hint", func() {
					diskSettings := DiskSettings{ID: "2", DeviceID: "def"}
					existingHintsOnDisk = make(map[string]DiskSettings)
					existingHintsOnDisk["2"] = diskSettings

					err := service.SavePersistentDiskHint(diskSettings)
					Expect(err).NotTo(HaveOccurred())

					jsonString, err := json.Marshal(existingHintsOnDisk)
					Expect(err).NotTo(HaveOccurred())

					fileContent, err := fs.ReadFile("/setting/persistent_hints.json")
					Expect(err).NotTo(HaveOccurred())
					Expect(fileContent).To(Equal(jsonString))
				})
			})

			Context("persistent disk hints file exists", func() {
				Context("has invalid hints saved on disk", func() {
					var existingHintsOnDisk []DiskSettings // The correct format is map[string]DiskSettings but we want to write out an incorrect format.

					BeforeEach(func() {
						service, fs = buildService()
						existingHintsOnDisk = []DiskSettings{
							{ID: "1", Path: "abc"},
							{ID: "2", Path: "def"},
							{ID: "3", Path: "ghi"},
						}
						jsonString, err := json.Marshal(existingHintsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						err = fs.WriteFileQuietly("/setting/persistent_hints.json", jsonString)
						Expect(err).ToNot(HaveOccurred())
					})

					It("returns error", func() {
						diskSettings := DiskSettings{ID: "2", DeviceID: "def"}
						err := service.SavePersistentDiskHint(diskSettings)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Unmarshalling persistent disk hints from file"))
					})
				})

				Context("has valid hints saved on disk", func() {
					var existingHintsOnDisk map[string]DiskSettings

					BeforeEach(func() {
						existingHintsOnDisk = map[string]DiskSettings{"1": {ID: "1", Path: "abc"}}
						jsonString, err := json.Marshal(existingHintsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						err = fs.WriteFileQuietly("/setting/persistent_hints.json", jsonString)
						Expect(err).ToNot(HaveOccurred())
					})

					It("reads and updates persistent disk hints on disk with provided hint", func() {
						diskSettings := DiskSettings{ID: "2", DeviceID: "def"}
						err := service.SavePersistentDiskHint(diskSettings)
						Expect(err).NotTo(HaveOccurred())

						existingHintsOnDisk["2"] = diskSettings
						jsonString, err := json.Marshal(existingHintsOnDisk)
						Expect(err).NotTo(HaveOccurred())

						fileContent, err := fs.ReadFile("/setting/persistent_hints.json")
						Expect(err).NotTo(HaveOccurred())
						Expect(fileContent).To(Equal(jsonString))
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
					Expect(fakeDefaultNetworkResolver.GetDefaultNetworkCalled).To(BeFalse())
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
						fakeDefaultNetworkResolver.GetDefaultNetworkNetwork = Network{
							IP:      "fake-resolved-ip",
							Netmask: "fake-resolved-netmask",
							Gateway: "fake-resolved-gateway",
						}
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
						fakeDefaultNetworkResolver.GetDefaultNetworkErr = errors.New("fake-get-default-network-err")
					})

					It("returns error", func() {
						settings := service.GetSettings()
						Expect(settings).To(Equal(loadedSettings))
					})
				})
			})
		})
	})
}
