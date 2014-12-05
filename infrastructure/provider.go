package infrastructure

import (
	"path/filepath"
	"time"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

type Provider struct {
	infrastructures map[string]Infrastructure
}

type ProviderOptions struct {
	MetadataService MetadataServiceOptions
}

func NewProvider(logger boshlog.Logger, platform boshplatform.Platform, options ProviderOptions) (p Provider) {
	fs := platform.GetFs()
	runner := platform.GetRunner()
	dirProvider := platform.GetDirProvider()

	mappedDevicePathResolver := boshdpresolv.NewMappedDevicePathResolver(500*time.Millisecond, fs)
	vsphereDevicePathResolver := boshdpresolv.NewVsphereDevicePathResolver(500*time.Millisecond, fs)
	dummyDevicePathResolver := boshdpresolv.NewDummyDevicePathResolver()

	resolver := NewRegistryEndpointResolver(
		NewDigDNSResolver(runner, logger),
	)

	awsMetadataService := NewAwsMetadataServiceProvider(resolver).Get()
	awsRegistry := NewAwsRegistry(awsMetadataService)

	awsInfrastructure := NewAwsInfrastructure(
		awsMetadataService,
		awsRegistry,
		platform,
		mappedDevicePathResolver,
		logger,
	)

	openstackMetadataService := NewOpenstackMetadataServiceProvider(resolver, platform, options.MetadataService, logger).Get()
	openstackRegistry := NewOpenstackRegistry(openstackMetadataService)

	openstackInfrastructure := NewOpenstackInfrastructure(
		openstackMetadataService,
		openstackRegistry,
		platform,
		mappedDevicePathResolver,
		logger,
	)

	wardenMetadataFilePath := filepath.Join(dirProvider.BoshDir(), "warden-cpi-metadata.json")
	wardenUserDataFilePath := filepath.Join(dirProvider.BoshDir(), "warden-cpi-user-data.json")
	wardenFallbackFileRegistryPath := filepath.Join(dirProvider.BoshDir(), "warden-cpi-agent-env.json")
	wardenMetadataService := NewFileMetadataService(wardenUserDataFilePath, wardenMetadataFilePath, fs, logger)
	wardenRegistryProvider := NewRegistryProvider(wardenMetadataService, wardenFallbackFileRegistryPath, fs, logger)

	p.infrastructures = map[string]Infrastructure{
		"aws":       awsInfrastructure,
		"openstack": openstackInfrastructure,
		"dummy":     NewDummyInfrastructure(fs, dirProvider, platform, dummyDevicePathResolver),
		"warden":    NewWardenInfrastructure(platform, dummyDevicePathResolver, wardenRegistryProvider),
		"vsphere":   NewVsphereInfrastructure(platform, vsphereDevicePathResolver, logger),
	}
	return
}

func (p Provider) Get(name string) (Infrastructure, error) {
	inf, found := p.infrastructures[name]
	if !found {
		return nil, bosherr.Errorf("Infrastructure %s could not be found", name)
	}
	return inf, nil
}
