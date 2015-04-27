package infrastructure

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type FileSettingsSource struct {
	metaDataFilePath string
	userDataFilePath string
	settingsFilePath string

	fs boshsys.FileSystem

	logger boshlog.Logger
	logTag string
}

func NewFileSettingsSource(
	metaDataFilePath string,
	userDataFilePath string,
	settingsFilePath string,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) *FileSettingsSource {
	return &FileSettingsSource{
		metaDataFilePath: metaDataFilePath,
		userDataFilePath: userDataFilePath,
		settingsFilePath: settingsFilePath,

		fs: fs,

		logTag: "FileSettingsSource",
		logger: logger,
	}
}

func (s *FileSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return "", nil
}

func (s *FileSettingsSource) Settings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings

	contents, err := s.fs.ReadFile(s.settingsFilePath)
	if err != nil {
		return settings, bosherr.WrapErrorf(
			err, "Reading from file '%s'", s.settingsFilePath)
	}

	err = json.Unmarshal(contents, &settings)
	if err != nil {
		return settings, bosherr.WrapErrorf(
			err, "Parsing file settings from '%s'", s.settingsFilePath)
	}

	return settings, nil
}
