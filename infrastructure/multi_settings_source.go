package infrastructure

import (
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type MultiSettingsSource struct {
	sources        []SettingsSource
	selectedSource SettingsSource
}

func NewMultiSettingsSource(sources ...SettingsSource) (SettingsSource, error) {
	var err error

	if len(sources) == 0 {
		err = bosherr.Error("MultiSettingsSource requires to have at least one source")
	}

	return &MultiSettingsSource{sources: sources}, err
}

func (s *MultiSettingsSource) PublicSSHKeyForUsername(username string) (string, error) {
	return s.getSelectedSource().PublicSSHKeyForUsername(username)
}

func (s *MultiSettingsSource) Settings() (boshsettings.Settings, error) {
	return s.getSelectedSource().Settings()
}

func (s *MultiSettingsSource) getSelectedSource() SettingsSource {
	if s.selectedSource != nil {
		return s.selectedSource
	}

	for _, source := range s.sources {
		_, err := source.Settings()
		if err == nil {
			s.selectedSource = source
			return s.selectedSource
		}
	}

	return s.sources[0]
}
