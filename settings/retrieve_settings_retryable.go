package settings

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type RetrieveSettingsRetryable interface {
	Attempt() (bool, error)
	Settings() Settings
}

type retrieveSettingsRetryable struct {
	source   Source
	settings Settings
	logger   boshlog.Logger
}

func NewRetrieveSettingsRetryable(source Source, logger boshlog.Logger) RetrieveSettingsRetryable {
	return &retrieveSettingsRetryable{
		source: source,
		logger: logger,
	}
}

func (s *retrieveSettingsRetryable) Attempt() (bool, error) {
	var fetchErr error
	s.logger.Debug("retrieveSettingsRetryable", "Fetching settings")
	s.settings, fetchErr = s.source.Settings()

	if fetchErr != nil {
		s.logger.Error("retrieveSettingsRetryable", "Fetching settings failed: ", fetchErr)
	} else {
		s.logger.Debug("retrieveSettingsRetryable", "Settings fetched successfully")
	}

	return true, fetchErr
}

func (s *retrieveSettingsRetryable) Settings() Settings {
	return s.settings
}
