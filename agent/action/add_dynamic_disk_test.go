package action

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
)

var _ = Describe("AddDynamicDiskAction", func() {
	var (
		action          AddDynamicDiskAction
		settingsService *fakesettings.FakeSettingsService
		platform        *platformfakes.FakePlatform
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		platform = &platformfakes.FakePlatform{}
		action = NewAddDynamicDiskAction(settingsService, platform)
	})

	It("sets up dynamic disk", func() {
		result, err := action.Run("diskCID", "/dev/sdb")

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]string{}))
		Expect(platform.SetupDynamicDiskCallCount()).To(Equal(1))
		Expect(platform.SetupDynamicDiskArgsForCall(0)).To(Equal(settings.DiskSettings{
			ID:       "diskCID",
			Path:     "/dev/sdb",
			VolumeID: "/dev/sdb",
		}))
	})

	Context("when setting up dynamic disk fails", func() {
		BeforeEach(func() {
			platform.SetupDynamicDiskReturns(errors.New("Could not setup"))
		})

		It("should raise error", func() {
			_, err := action.Run("diskCID", "/dev/sdb")
			Expect(err).To(HaveOccurred())
		})
	})
})
