package infrastructure

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type fileRegistry struct {
	registryFilePath string
	fs               boshsys.FileSystem
}

func NewFileRegistry(registryFilePath string, fs boshsys.FileSystem) Registry {
	return &fileRegistry{
		registryFilePath: registryFilePath,
		fs:               fs,
	}
}

func (r *fileRegistry) GetSettings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings

	contents, err := r.fs.ReadFile(r.registryFilePath)
	if err != nil {
		return settings, bosherr.WrapError(err, "Read settings file")
	}

	err = json.Unmarshal([]byte(contents), &settings)
	if err != nil {
		return settings, bosherr.WrapError(err, "Unmarshal json settings")
	}

	return settings, nil
}
