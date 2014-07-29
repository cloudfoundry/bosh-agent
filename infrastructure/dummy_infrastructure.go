package infrastructure

import (
	"encoding/json"
	"path/filepath"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type dummyInfrastructure struct {
	fs                 boshsys.FileSystem
	dirProvider        boshdir.Provider
	platform           boshplatform.Platform
	devicePathResolver boshdpresolv.DevicePathResolver
}

func NewDummyInfrastructure(
	fs boshsys.FileSystem,
	dirProvider boshdir.Provider,
	platform boshplatform.Platform,
	devicePathResolver boshdpresolv.DevicePathResolver,
) (inf dummyInfrastructure) {
	inf.fs = fs
	inf.dirProvider = dirProvider
	inf.platform = platform
	inf.devicePathResolver = devicePathResolver
	return
}

func (inf dummyInfrastructure) GetDevicePathResolver() boshdpresolv.DevicePathResolver {
	return inf.devicePathResolver
}

func (inf dummyInfrastructure) SetupSSH(username string) error {
	return nil
}

func (inf dummyInfrastructure) GetSettings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings

	// dummy-cpi-agent-env.json is written out by dummy CPI.
	settingsPath := filepath.Join(inf.dirProvider.BoshDir(), "dummy-cpi-agent-env.json")
	contents, err := inf.fs.ReadFile(settingsPath)
	if err != nil {
		return settings, bosherr.WrapError(err, "Read settings file")
	}

	err = json.Unmarshal([]byte(contents), &settings)
	if err != nil {
		return settings, bosherr.WrapError(err, "Unmarshal json settings")
	}

	return settings, nil
}

func (inf dummyInfrastructure) SetupNetworking(networks boshsettings.Networks) error {
	return nil
}

func (inf dummyInfrastructure) GetEphemeralDiskPath(devicePath string) (string, bool) {
	return inf.platform.NormalizeDiskPath(devicePath)
}
