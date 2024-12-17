package infrastructure

import (
	"encoding/json"
	"fmt"
	"time"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type InstanceMetadataSettingsSource struct {
	metadataHost    string
	metadataHeaders map[string]string
	settingsPath    string

	platform boshplatform.Platform
	logger   boshlog.Logger

	logTag          string
	metadataService HTTPMetadataService
}

func NewInstanceMetadataSettingsSource(
	metadataHost string,
	metadataHeaders map[string]string,
	settingsPath string,
	platform boshplatform.Platform,
	logger boshlog.Logger,
) *InstanceMetadataSettingsSource {
	logTag := "InstanceMetadataSettingsSource"
	return &InstanceMetadataSettingsSource{
		metadataHost:    metadataHost,
		metadataHeaders: metadataHeaders,
		settingsPath:    settingsPath,

		platform: platform,
		logger:   logger,

		logTag: logTag,
		// The HTTPMetadataService provides more functionality than we need (like custom DNS), so we
		// pass zero values to the New function and only use its GetValueAtPath method.
		metadataService: NewHTTPMetadataService(metadataHost, metadataHeaders, "", "", "", "", platform, logger),
	}
}

func NewInstanceMetadataSettingsSourceWithoutRetryDelay(
	metadataHost string,
	metadataHeaders map[string]string,
	settingsPath string,
	platform boshplatform.Platform,
	logger boshlog.Logger,
) *InstanceMetadataSettingsSource {
	logTag := "InstanceMetadataSettingsSource"
	return &InstanceMetadataSettingsSource{
		metadataHost:    metadataHost,
		metadataHeaders: metadataHeaders,
		settingsPath:    settingsPath,

		platform: platform,
		logger:   logger,

		logTag: logTag,
		// The HTTPMetadataService provides more functionality than we need (like custom DNS), so we
		// pass zero values to the New function and only use its GetValueAtPath method.
		metadataService: NewHTTPMetadataServiceWithCustomRetryDelay(metadataHost, metadataHeaders, "", "", "", platform, logger, 0*time.Second),
	}
}

func (s InstanceMetadataSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return "", nil
}

func (s *InstanceMetadataSettingsSource) Settings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings
	contents, err := s.metadataService.GetValueAtPath(s.settingsPath)
	if err != nil {
		return settings, bosherr.WrapError(err, fmt.Sprintf("Reading settings from instance metadata at path %q", s.settingsPath))
	}

	err = json.Unmarshal([]byte(contents), &settings)
	if err != nil {
		return settings, bosherr.WrapErrorf(
			err, "Parsing instance metadata settings from %q", contents)
	}

	return settings, nil
}
