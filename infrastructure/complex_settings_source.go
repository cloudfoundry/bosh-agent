package infrastructure

import (
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type ComplexSettingsSource struct {
	metadataService  MetadataService
	registryProvider RegistryProvider

	logTag string
	logger boshlog.Logger
}

func NewComplexSettingsSource(
	metadataService MetadataService,
	registryProvider RegistryProvider,
	logger boshlog.Logger,
) ComplexSettingsSource {
	return ComplexSettingsSource{
		metadataService:  metadataService,
		registryProvider: registryProvider,

		logTag: "ComplexSettingsSource",
		logger: logger,
	}
}

func (s ComplexSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return s.metadataService.GetPublicKey()
}

func (s ComplexSettingsSource) Settings() (boshsettings.Settings, error) {
	registry, err := s.registryProvider.GetRegistry()
	if err != nil {
		return boshsettings.Settings{}, err
	}

	return registry.GetSettings()
}
