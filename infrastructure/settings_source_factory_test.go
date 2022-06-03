package infrastructure_test

import (
	. "github.com/cloudfoundry/bosh-agent/infrastructure"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"reflect"

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
