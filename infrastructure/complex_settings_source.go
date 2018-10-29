package infrastructure

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
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
	settings, err := s.GetMetadataService().GetSettings()
	if err == nil && settings.AgentID != "" {
		s.logger.Debug(s.logTag, "Got settings from metadata service, not contacting registry.")
		return settings, nil
	}

	s.logger.Debug(s.logTag, "Unable to get settings from metadata service, falling back to registry.")

	registry, err := s.registryProvider.GetRegistry()
	if err != nil {
		return boshsettings.Settings{}, err
	}

	return registry.GetSettings()
}

func (s ComplexSettingsSource) GetMetadataService() MetadataService {
	return s.metadataService
}
