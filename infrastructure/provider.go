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

func NewProvider(logger boshlog.Logger, platform boshplatform.Platform) (p Provider) {
	fs := platform.GetFs()
	dirProvider := platform.GetDirProvider()

	mappedDevicePathResolver := boshdpresolv.NewMappedDevicePathResolver(500*time.Millisecond, fs)
	vsphereDevicePathResolver := boshdpresolv.NewVsphereDevicePathResolver(500*time.Millisecond, fs)
	dummyDevicePathResolver := boshdpresolv.NewDummyDevicePathResolver()

	resolver := NewRegistryEndpointResolver(
		NewDigDNSResolver(logger),
	)

	awsMetadataServiceProvider := NewAwsMetadataServiceProvider(resolver)
	awsMetadataService := awsMetadataServiceProvider.GetMetadataService()
	awsRegistry := NewAwsRegistry(awsMetadataService)

	awsInfrastructure := NewAwsInfrastructure(
		awsMetadataService,
		awsRegistry,
		platform,
		mappedDevicePathResolver,
		logger,
	)

	openstackMetadataServiceProvider := NewOpenstackMetadataServiceProvider(resolver)
	openstackMetadataService := openstackMetadataServiceProvider.GetMetadataService()
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
