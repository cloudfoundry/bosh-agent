package infrastructure

import (
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

const openstackInfrastructureLogTag = "openstackInfrastructure"

type openstackInfrastructure struct {
	metadataService    MetadataService
	registry           Registry
	platform           boshplatform.Platform
	devicePathResolver boshdpresolv.DevicePathResolver
	logger             boshlog.Logger
}

func NewOpenstackInfrastructure(
	metadataService MetadataService,
	registry Registry,
	platform boshplatform.Platform,
	devicePathResolver boshdpresolv.DevicePathResolver,
	logger boshlog.Logger,
) openstackInfrastructure {

	return openstackInfrastructure{
		metadataService:    metadataService,
		registry:           registry,
		platform:           platform,
		devicePathResolver: devicePathResolver,
		logger:             logger,
	}
}

func NewOpenstackRegistry(metadataService MetadataService) Registry {
	return NewConcreteRegistry(metadataService, true)
}

func (inf openstackInfrastructure) GetDevicePathResolver() boshdpresolv.DevicePathResolver {
	return inf.devicePathResolver
}

func (inf openstackInfrastructure) SetupSSH(username string) error {
	publicKey, err := inf.metadataService.GetPublicKey()
	if err != nil {
		return bosherr.WrapError(err, "Error getting public key")
	}

	return inf.platform.SetupSSH(publicKey, username)
}

func (inf openstackInfrastructure) GetSettings() (boshsettings.Settings, error) {
	registry := inf.registry
	settings, err := registry.GetSettings()
	if err != nil {
		return settings, bosherr.WrapError(err, "Getting settings from registry")
	}

	return settings, nil
}

func (inf openstackInfrastructure) SetupNetworking(networks boshsettings.Networks) (err error) {
	return inf.platform.SetupDhcp(networks)
}

func (inf openstackInfrastructure) GetEphemeralDiskPath(devicePath string) string {
	if devicePath == "" {
		inf.logger.Info(openstackInfrastructureLogTag, "Ephemeral disk path is empty")
		return ""
	}

	return inf.platform.NormalizeDiskPath(devicePath)
}
