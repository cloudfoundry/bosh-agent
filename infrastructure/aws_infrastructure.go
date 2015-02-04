package infrastructure

import (
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

const awsInfrastructureLogTag = "awsInfrastructure"

type awsInfrastructure struct {
	metadataService    MetadataService
	registry           Registry
	platform           boshplatform.Platform
	devicePathResolver boshdpresolv.DevicePathResolver
	useServerNameAsID  bool
	logger             boshlog.Logger
}

func NewAwsInfrastructure(
	metadataService MetadataService,
	registry Registry,
	platform boshplatform.Platform,
	devicePathResolver boshdpresolv.DevicePathResolver,
	logger boshlog.Logger,
) awsInfrastructure {
	return awsInfrastructure{
		metadataService:    metadataService,
		registry:           registry,
		platform:           platform,
		devicePathResolver: devicePathResolver,
		logger:             logger,
	}
}

func NewAwsRegistry(metadataService MetadataService) Registry {
	return NewHTTPRegistry(metadataService, false)
}

func (inf awsInfrastructure) GetDevicePathResolver() boshdpresolv.DevicePathResolver {
	return inf.devicePathResolver
}

func (inf awsInfrastructure) SetupSSH(username string) error {
	publicKey, err := inf.metadataService.GetPublicKey()
	if err != nil {
		return bosherr.WrapError(err, "Error getting public key")
	}

	return inf.platform.SetupSSH(publicKey, username)
}

func (inf awsInfrastructure) GetSettings() (boshsettings.Settings, error) {
	registry := inf.registry
	settings, err := registry.GetSettings()
	if err != nil {
		return settings, bosherr.WrapError(err, "Getting settings from registry")
	}

	return settings, nil
}

func (inf awsInfrastructure) SetupNetworking(networks boshsettings.Networks) (err error) {
	return inf.platform.SetupDhcp(networks)
}

func (inf awsInfrastructure) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string {
	if diskSettings.Path == "" {
		inf.logger.Info(awsInfrastructureLogTag, "Ephemeral disk path is empty")
		return ""
	}

	return inf.platform.NormalizeDiskPath(diskSettings)
}
