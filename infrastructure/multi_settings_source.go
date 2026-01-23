package infrastructure

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type MultiSettingsSource struct {
	sources                []boshsettings.Source
	selectedSSHKeySource   boshsettings.Source
	selectedSettingsSource boshsettings.Source
	logger                 boshlog.Logger
}

func NewMultiSettingsSource(logger boshlog.Logger, sources ...boshsettings.Source) (boshsettings.Source, error) {
	var err error

	if len(sources) == 0 {
		err = bosherr.Error("MultiSettingsSource requires to have at least one source")
	}

	return &MultiSettingsSource{sources: sources, logger: logger}, err
}

func (s *MultiSettingsSource) GetSources() []boshsettings.Source {
	return s.sources
}

func (s *MultiSettingsSource) PublicSSHKeyForUsername(username string) (string, error) {
	if s.selectedSSHKeySource != nil {
		return s.selectedSSHKeySource.PublicSSHKeyForUsername(username)
	}

	var publicSSHKey string
	var err error

	for _, source := range s.sources {
		publicSSHKey, err = source.PublicSSHKeyForUsername(username)
		if err == nil {
			s.selectedSSHKeySource = source
			return publicSSHKey, nil
		}
	}

	return "", bosherr.WrapErrorf(err, "Getting public SSH key for '%s'", username)
}

func (s *MultiSettingsSource) Settings() (boshsettings.Settings, error) {
	if s.selectedSettingsSource != nil {
		return s.selectedSettingsSource.Settings()
	}

	var settings boshsettings.Settings
	var err error

	for _, source := range s.sources {
		settings, err = source.Settings()
		if err == nil {
			s.selectedSettingsSource = source
			return settings, nil
		}
		s.logger.Warn("multi-settings-source", "Failed to get settings from source: %v", err)
	}

	return boshsettings.Settings{},
		bosherr.WrapError(err, "Getting settings from all sources")
}
