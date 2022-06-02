package infrastructure_test

import (
	"reflect"

	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
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

					metadataService := settingsSource.(*MultiSettingsSource).GetMetadataService()
					httpMetadataService := metadataService.(*MultiSourceMetadataService).Services[0]

					Expect(reflect.TypeOf(httpMetadataService).Name()).To(Equal(reflect.TypeOf(HTTPMetadataService{}).Name()))
				})
			})

			Context("when using ConfigDrive source", func() {
				BeforeEach(func() {
					options.Sources = []SourceOptions{
						ConfigDriveSourceOptions{
							DiskPaths: []string{"/fake-disk-path"},

							MetaDataPath: "fake-meta-data-path",
							UserDataPath: "fake-user-data-path",

							SettingsPath: "fake-settings-path",
						},
					}
				})
				// was only used when registry is set to true
				//			It("returns a settings source that uses config drive to fetch settings", func() {
				//				configDriveMetadataService := NewConfigDriveMetadataService(
				//					platform,
				//					[]string{"/fake-disk-path"},
				//					"fake-meta-data-path",
				//					"fake-user-data-path",
				//					logger,
				//				)
				//				multiSourceMetadataService := NewMultiSourceMetadataService(configDriveMetadataService)
				//				configDriveSettingsSource := NewComplexSettingsSource(multiSourceMetadataService, logger)

				//				settingsSource, err := factory.New()
				//				Expect(err).ToNot(HaveOccurred())
				//				Expect(settingsSource).To(Equal(configDriveSettingsSource))
				//			})
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

				// was only used when registry is set to true
				//			It("returns a settings source that uses file to fetch settings", func() {
				//				fileMetadataService := NewFileMetadataService(
				//					"fake-meta-data-path",
				//					"fake-user-data-path",
				//					"fake-settings-path",
				//					platform.GetFs(),
				//					logger,
				//				)
				//				multiSourceMetadataService := NewMultiSourceMetadataService(fileMetadataService)
				//				fileSettingsSource := NewComplexSettingsSource(multiSourceMetadataService, logger)

				//				settingsSource, err := factory.New()
				//				Expect(err).ToNot(HaveOccurred())
				//				Expect(settingsSource).To(Equal(fileSettingsSource))
				//			})
			})

			Context("when using ConfigDrive source", func() {
				BeforeEach(func() {
					options = SettingsOptions{
						Sources: []SourceOptions{
							ConfigDriveSourceOptions{
								DiskPaths:    []string{"/fake-disk-path"},
								MetaDataPath: "fake-meta-data-path",
								SettingsPath: "fake-settings-path",
							},
						},
					}
				})

				FIt("returns a settings source that uses config drive to fetch settings", func() {

					configDriveSettingsSource := NewConfigDriveSettingsSource(
						[]string{"/fake-disk-path"},
						"fake-meta-data-path",
						"fake-settings-path",
						platform,
						logger,
					)

					multiSettingsSource, err := NewMultiSettingsSource(configDriveSettingsSource)
					Expect(err).ToNot(HaveOccurred())
					settingsSource, err := factory.New()

					Expect(err).ToNot(HaveOccurred())
					Expect(settingsSource).To(Equal(multiSettingsSource))
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

					multiSettingsSource, err := NewMultiSettingsSource(fileSettingsSource)
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

					multiSettingsSource, err := NewMultiSettingsSource(cdromSettingsSource)
					Expect(err).ToNot(HaveOccurred())

					settingsSource, err := factory.New()
					Expect(err).ToNot(HaveOccurred())
					Expect(settingsSource).To(Equal(multiSettingsSource))
				})
			})
		})
	})
})
