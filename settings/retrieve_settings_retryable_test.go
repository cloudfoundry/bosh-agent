package settings_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/infrastructure/fakes"
	fakeLogger "github.com/cloudfoundry/bosh-agent/logger/fakes"
	. "github.com/cloudfoundry/bosh-agent/settings"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RetrieveSettingsRetryable", func() {
	Describe("Attempt", func() {
		var (
			source            *fakes.FakeSettingsSource
			settingsRetryable RetrieveSettingsRetryable
			logger            *fakeLogger.FakeLogger
		)

		BeforeEach(func() {
			source = &fakes.FakeSettingsSource{}
			logger = &fakeLogger.FakeLogger{}
			settingsRetryable = NewRetrieveSettingsRetryable(source, logger)
		})

		Context("when settings are retrieved", func() {
			BeforeEach(func() {
				source.SettingsValue = Settings{AgentID: "some-agend-id"}
			})

			It("it provides the settings", func() {
				_, err := settingsRetryable.Attempt()
				Expect(err).ToNot(HaveOccurred())

				settings := settingsRetryable.Settings()
				Expect(settings).To(Equal(Settings{AgentID: "some-agend-id"}))
			})

			It("is still retryable", func() {
				retryable, _ := settingsRetryable.Attempt()
				Expect(retryable).To(Equal(true))
			})

			It("logs an attempt", func() {
				_, _ = settingsRetryable.Attempt()
				Expect(logger.DebugCallCount()).To(Equal(2))
				tag, msg, _ := logger.DebugArgsForCall(0)
				Expect(tag).To(Equal("retrieveSettingsRetryable"))
				Expect(msg).To(Equal("Fetching settings"))

				_, msg, _ = logger.DebugArgsForCall(1)
				Expect(msg).To(Equal("Settings fetched successfully"))
			})
		})

		Context("when settings cannot be retrieved", func() {
			BeforeEach(func() {
				source.SettingsErr = errors.New("fake-settings-fetch-error")
			})

			It("is returns an error", func() {
				_, err := settingsRetryable.Attempt()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-settings-fetch-error"))
			})

			It("is retryable", func() {
				retryable, _ := settingsRetryable.Attempt()
				Expect(retryable).To(Equal(true))
			})

			It("logs an error", func() {
				_, _ = settingsRetryable.Attempt()
				Expect(logger.ErrorCallCount()).To(Equal(1))
				tag, msg, _ := logger.ErrorArgsForCall(0)
				Expect(tag).To(Equal("retrieveSettingsRetryable"))
				Expect(msg).To(ContainSubstring("Fetching settings failed: "))
			})
		})
	})
})
