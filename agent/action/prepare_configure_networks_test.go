package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
)

var _ = Describe("prepareConfigureNetworks", func() {
	var (
		prepareConfigureNetworksAction action.PrepareConfigureNetworksAction
		platform                       *platformfakes.FakePlatform
		settingsService                *fakesettings.FakeSettingsService
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		settingsService = &fakesettings.FakeSettingsService{}
		prepareConfigureNetworksAction = action.NewPrepareConfigureNetworks(platform, settingsService)
	})

	AssertActionIsNotAsynchronous(prepareConfigureNetworksAction)
	AssertActionIsNotPersistent(prepareConfigureNetworksAction)
	AssertActionIsLoggable(prepareConfigureNetworksAction)

	AssertActionIsNotResumable(prepareConfigureNetworksAction)
	AssertActionIsNotCancelable(prepareConfigureNetworksAction)

	Describe("Run", func() {
		It("invalidates settings so that load settings cannot fall back on old settings", func() {
			resp, err := prepareConfigureNetworksAction.Run()
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).To(Equal("ok"))

			Expect(settingsService.SettingsWereInvalidated).To(BeTrue())
		})

		Context("when settings invalidation succeeds", func() {
			It("prepares platform for networking change", func() {
				resp, err := prepareConfigureNetworksAction.Run()
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).To(Equal("ok"))

				Expect(platform.PrepareForNetworkingChangeCallCount()).To(Equal(1))
			})

			Context("when preparing for networking change fails", func() {
				BeforeEach(func() {
					platform.PrepareForNetworkingChangeReturns(errors.New("fake-prepare-error"))
				})

				It("returns error if preparing for networking change fails", func() {
					resp, err := prepareConfigureNetworksAction.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-prepare-error"))
					Expect(resp).To(Equal(""))
				})
			})
		})

		Context("when settings invalidation fails", func() {
			BeforeEach(func() {
				settingsService.InvalidateSettingsError = errors.New("fake-invalidate-error")
			})

			It("returns error early if settings err invalidating", func() {
				resp, err := prepareConfigureNetworksAction.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-invalidate-error"))

				Expect(resp).To(Equal(""))
			})

			It("does not prepare platform for networking change", func() {
				_, err := prepareConfigureNetworksAction.Run()
				Expect(err).To(HaveOccurred())

				Expect(platform.PrepareForNetworkingChangeCallCount()).To(Equal(0))
			})
		})
	})
})
