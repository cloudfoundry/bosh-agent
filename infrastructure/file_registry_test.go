package infrastructure_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/infrastructure"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("FileRegistry", func() {
	var (
		fs           *fakesys.FakeFileSystem
		fileRegistry infrastructure.Registry
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		fileRegistry = infrastructure.NewFileRegistry("/fake-registry-file-path", fs)
	})

	Describe("GetSettings", func() {
		Context("when the registry file exists", func() {
			var (
				expectedSettings boshsettings.Settings
			)

			BeforeEach(func() {
				expectedSettings = boshsettings.Settings{
					AgentID: "fake-agent-id",
				}
				settingsJSON, err := json.Marshal(expectedSettings)
				Expect(err).ToNot(HaveOccurred())

				err = fs.WriteFile("/fake-registry-file-path", settingsJSON)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the settings", func() {
				settings, err := fileRegistry.GetSettings()
				Expect(err).ToNot(HaveOccurred())
				Expect(settings).To(Equal(expectedSettings))
			})
		})

		Context("when the registry file does not exist", func() {
			It("returns an error", func() {
				_, err := fileRegistry.GetSettings()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
