package infrastructure

import (
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
	dirProvider := platform.GetDirProvider()

	mappedDevicePathResolver := boshdpresolv.NewMappedDevicePathResolver(500*time.Millisecond, fs)
	vsphereDevicePathResolver := boshdpresolv.NewVsphereDevicePathResolver(500*time.Millisecond, fs)
	dummyDevicePathResolver := boshdpresolv.NewDummyDevicePathResolver()

	resolver := NewRegistryEndpointResolver(
		NewDigDNSResolver(logger),
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

	p.infrastructures = map[string]Infrastructure{
		"aws":       awsInfrastructure,
		"openstack": openstackInfrastructure,
		"dummy":     NewDummyInfrastructure(fs, dirProvider, platform, dummyDevicePathResolver),
		"warden":    NewWardenInfrastructure(dirProvider, platform, dummyDevicePathResolver),
		"vsphere":   NewVsphereInfrastructure(platform, vsphereDevicePathResolver, logger),
	}
	return
}

func (p Provider) Get(name string) (Infrastructure, error) {
	inf, found := p.infrastructures[name]
	if !found {
		return nil, bosherr.New("Infrastructure %s could not be found", name)
	}
	return inf, nil
}
