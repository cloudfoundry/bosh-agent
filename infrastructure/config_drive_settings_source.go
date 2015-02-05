package infrastructure

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type MetadataContentsType struct {
	PublicKeys map[string]PublicKeyType `json:"public-keys"`
	InstanceID string                   `json:"instance-id"` // todo remove
}

type PublicKeyType map[string]string

type ConfigDriveSettingsSource struct {
	diskPaths    []string
	metadataPath string
	settingsPath string

	platform boshplatform.Platform

	logTag string
	logger boshlog.Logger
}

func NewConfigDriveSettingsSource(
	diskPaths []string,
	metadataPath string,
	settingsPath string,
	platform boshplatform.Platform,
	logger boshlog.Logger,
) *ConfigDriveSettingsSource {
	return &ConfigDriveSettingsSource{
		diskPaths:    diskPaths,
		metadataPath: metadataPath,
		settingsPath: settingsPath,

		platform: platform,

		logTag: "ConfigDriveSettingsSource",
		logger: logger,
	}
}

func (s *ConfigDriveSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	err := s.loadIfNecessary()
	if err != nil {
		return "", err
	}

	if firstPublicKey, ok := s.metadata.PublicKeys["0"]; ok {
		if openSSHKey, ok := firstPublicKey["openssh-key"]; ok {
			return openSSHKey, nil
		}
	}

	return "", nil
}

func (s *ConfigDriveSettingsSource) Settings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings

	contents, err := s.platform.GetFilesContentsFromDisk(
		s.diskPaths[0], // todo
		[]string{s.metadataPath, s.settingsPath},
	)
	if err != nil {
		return settings, bosherr.WrapError(err, "Reading files on config drive")
	}

	err = json.Unmarshal(contents[1], &settings)
	if err != nil {
		return settings, bosherr.WrapErrorf(
			err, "Parsing config drive settings from '%s'", s.settingsPath)
	}

	var metadata MetadataContentsType
	err = json.Unmarshal(contents[0], &metadata)
	if err != nil {
		return settings, bosherr.WrapErrorf(err, "Parsing config drive metadata from '%s'", s.metadataPath)
	}

	return settings, nil
}
