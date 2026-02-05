package infrastructure_test

import (
	"encoding/json"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
)

var _ = Describe("SettingsSourceFactory", func() {
	Describe("New", func() {
		var (
			options  SettingsOptions
			platform *platformfakes.FakePlatform
			logger   boshlog.Logger
			factory  SettingsSourceFactory
		)

		BeforeEach(func() {
			options = SettingsOptions{}
			platform = &platformfakes.FakePlatform{}
			logger = boshlog.NewLogger(boshlog.LevelNone)
		})

		JustBeforeEach(func() {
			factory = NewSettingsSourceFactory(options, platform, logger)
		})

		Context("when using config sources", func() {

			Context("when using HTTP source", func() {
				BeforeEach(func() {
					options.Sources = []SourceOptions{
						HTTPSourceOptions{URI: "http://fake-url"},
					}
				})
				It("returns a settings source that uses HTTP to fetch settings", func() {
					settingsSource, err := factory.New()
					Expect(err).ToNot(HaveOccurred())
					sources := settingsSource.(*MultiSettingsSource).GetSources()
					Expect(len(sources)).To(Equal(1))
					Expect(reflect.TypeOf(sources[0]).Name()).To(Equal(reflect.TypeOf(HTTPMetadataService{}).Name()))
				})
			})

			Context("when using ConfigDrive source", func() {
				BeforeEach(func() {
					options.Sources = []SourceOptions{
						ConfigDriveSourceOptions{
							DiskPaths:    []string{"/fake-disk-path"},
							MetaDataPath: "fake-meta-data-path",
							SettingsPath: "fake-settings-path",
						},
					}
				})

				It("returns a settings source that uses config drive to fetch settings", func() {
					settingsSource, err := factory.New()
					Expect(err).ToNot(HaveOccurred())
					sources := settingsSource.(*MultiSettingsSource).GetSources()
					Expect(len(sources)).To(Equal(1))
					Expect(reflect.TypeOf(sources[0]).Elem().Name()).To(Equal(reflect.TypeOf(ConfigDriveSettingsSource{}).Name()))

				})
			})

			Context("when using File source", func() {
				BeforeEach(func() {
					options.Sources = []SourceOptions{
						FileSourceOptions{
							MetaDataPath: "fake-meta-data-path",
							UserDataPath: "fake-user-data-path",

							SettingsPath: "fake-settings-path",
						},
					}
				})

				It("returns a settings source that uses a file to fetch settings", func() {
					fileSettingsSource := NewFileSettingsSource(
						"fake-settings-path",
						platform.GetFs(),
						logger,
					)

					multiSettingsSource, err := NewMultiSettingsSource(logger, fileSettingsSource)
					Expect(err).ToNot(HaveOccurred())

					settingsSource, err := factory.New()
					Expect(err).ToNot(HaveOccurred())
					Expect(settingsSource).To(Equal(multiSettingsSource))
				})
			})

			Context("when using CDROM source", func() {
				BeforeEach(func() {
					options = SettingsOptions{
						Sources: []SourceOptions{
							CDROMSourceOptions{
								FileName: "fake-file-name",
							},
						},
					}
				})

				It("returns a settings source that uses the CDROM to fetch settings", func() {
					cdromSettingsSource := NewCDROMSettingsSource(
						"fake-file-name",
						platform,
						logger,
					)

					multiSettingsSource, err := NewMultiSettingsSource(logger, cdromSettingsSource)
					Expect(err).ToNot(HaveOccurred())

					settingsSource, err := factory.New()
					Expect(err).ToNot(HaveOccurred())
					Expect(settingsSource).To(Equal(multiSettingsSource))
				})
			})

			Context("when using VsphereGuestInfo source", func() {
				BeforeEach(func() {
					options = SettingsOptions{
						Sources: []SourceOptions{
							VsphereGuestInfoSourceOptions{},
						},
					}
				})

				It("returns a settings source that uses the VsphereGuestInfo to fetch settings", func() {
					vsphereGuestInfoSettingsSource := NewVsphereGuestInfoSettingsSource(platform, logger, "", "")

					multiSettingsSource, err := NewMultiSettingsSource(logger, vsphereGuestInfoSettingsSource)
					Expect(err).ToNot(HaveOccurred())

					settingsSource, err := factory.New()
					Expect(err).ToNot(HaveOccurred())
					Expect(settingsSource).To(Equal(multiSettingsSource))
				})
			})
		})
	})

	Describe("UnmarshalJSON", func() {
		var (
			sourceOptionsSlice SourceOptionsSlice
		)
		BeforeEach(func() {
			sourceOptionsSlice = SourceOptionsSlice{}
		})

		It("unmarshals HTTP source options", func() {
			jsonStr := `[{"Type": "HTTP", "URI": "http://example.com"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(HTTPSourceOptions{URI: "http://example.com"}))
		})

		It("unmarshals InstanceMetadata source options", func() {
			jsonStr := `[{"Type": "InstanceMetadata", "URI": "http://metadata.google.internal"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(InstanceMetadataSourceOptions{URI: "http://metadata.google.internal"}))
		})

		It("unmarshals ConfigDrive source options", func() {
			jsonStr := `[{"Type": "ConfigDrive", "DiskPaths": ["/dev/vdb"]}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(ConfigDriveSourceOptions{DiskPaths: []string{"/dev/vdb"}}))
		})

		It("unmarshals File source options", func() {
			jsonStr := `[{"Type": "File", "SettingsPath": "/tmp/settings.json"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(FileSourceOptions{SettingsPath: "/tmp/settings.json"}))
		})

		It("unmarshals CDROM source options", func() {
			jsonStr := `[{"Type": "CDROM", "FileName": "env"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(CDROMSourceOptions{FileName: "env"}))
		})

		It("unmarshals VsphereGuestInfo source options", func() {
			jsonStr := `[{"Type": "VsphereGuestInfo"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(VsphereGuestInfoSourceOptions{}))
		})

		It("unmarshals VsphereGuestInfo source options with custom tool paths", func() {
			jsonStr := `[{"Type": "VsphereGuestInfo", "RpcToolPath": "C:\\Program Files\\VMware\\VMware Tools\\rpctool.exe", "VmToolsdPath": "C:\\Program Files\\VMware\\VMware Tools\\vmtoolsd.exe"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceOptionsSlice).To(HaveLen(1))
			Expect(sourceOptionsSlice[0]).To(Equal(VsphereGuestInfoSourceOptions{
				RpcToolPath:  `C:\Program Files\VMware\VMware Tools\rpctool.exe`,
				VmToolsdPath: `C:\Program Files\VMware\VMware Tools\vmtoolsd.exe`,
			}))
		})

		It("returns error when Type is missing", func() {
			jsonStr := `[{"URI": "http://example.com"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Missing source type"))
		})

		It("returns error when Type is unknown", func() {
			jsonStr := `[{"Type": "Unknown"}]`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unknown source type 'Unknown'"))
		})

		It("returns error when JSON is invalid", func() {
			jsonStr := `invalid-json`
			err := json.Unmarshal([]byte(jsonStr), &sourceOptionsSlice)
			Expect(err).To(HaveOccurred())
		})
	})
})
