package infrastructure

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type CDROMSettingsSource struct {
	settingsFileName string

	loaded  bool
	loadErr error

	// Loaded state
	settings boshsettings.Settings

	platform boshplatform.Platform

	logTag string
	logger boshlog.Logger
}

func NewCDROMSettingsSource(
	settingsFileName string,
	platform boshplatform.Platform,
	logger boshlog.Logger,
) *CDROMSettingsSource {
	return &CDROMSettingsSource{
		settingsFileName: settingsFileName,

		platform: platform,

		logTag: "CDROMSettingsSource",
		logger: logger,
	}
}

func (s CDROMSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return "", nil
}

func (s *CDROMSettingsSource) Settings() (boshsettings.Settings, error) {
	err := s.loadIfNecessary()
	return s.settings, err
}

func (s *CDROMSettingsSource) loadIfNecessary() error {
	if !s.loaded {
		s.loaded = true
		s.loadErr = s.load()
	}

	return s.loadErr
}

func (s *CDROMSettingsSource) load() error {
	contents, err := s.platform.GetFileContentsFromCDROM(s.settingsFileName)
	if err != nil {
		return bosherr.WrapError(err, "Reading files from CDROM")
	}

	err = json.Unmarshal(contents, &s.settings)
	if err != nil {
		return bosherr.WrapErrorf(
			err, "Parsing CDROM settings from '%s'", s.settingsFileName)
	}

	return nil
}
