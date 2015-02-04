package infrastructure

import (
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type wardenInfrastructure struct {
	platform           boshplatform.Platform
	devicePathResolver boshdpresolv.DevicePathResolver
	registryProvider   RegistryProvider
}

func NewWardenInfrastructure(
	platform boshplatform.Platform,
	devicePathResolver boshdpresolv.DevicePathResolver,
	registryProvider RegistryProvider,
) wardenInfrastructure {
	return wardenInfrastructure{
		platform:           platform,
		devicePathResolver: devicePathResolver,
		registryProvider:   registryProvider,
	}
}

func (inf wardenInfrastructure) GetDevicePathResolver() boshdpresolv.DevicePathResolver {
	return inf.devicePathResolver
}

func (inf wardenInfrastructure) SetupSSH(username string) error {
	return nil
}

func (inf wardenInfrastructure) GetSettings() (boshsettings.Settings, error) {
	registry, _ := inf.registryProvider.GetRegistry() // todo
	return registry.GetSettings()
}

func (inf wardenInfrastructure) SetupNetworking(networks boshsettings.Networks) error {
	return nil
}

func (inf wardenInfrastructure) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string {
	return inf.platform.NormalizeDiskPath(diskSettings)
}
