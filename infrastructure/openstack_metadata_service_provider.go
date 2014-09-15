package infrastructure

import (
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

type openstackServiceProvider struct {
	resolver               DNSResolver
	platform               boshplatform.Platform
	metadataServiceOptions MetadataServiceOptions
}

func NewOpenstackMetadataServiceProvider(
	resolver DNSResolver,
	platform boshplatform.Platform,
	options MetadataServiceOptions,
) openstackServiceProvider {
	return openstackServiceProvider{
		resolver:               resolver,
		platform:               platform,
		metadataServiceOptions: options,
	}
}

func (inf openstackServiceProvider) Get() MetadataService {
	if inf.metadataServiceOptions.UseConfigDrive {
		configDriveDiskPaths := []string{
			"/dev/disk/by-label/CONFIG-2",
			"/dev/disk/by-label/config-2",
		}

		metadataService := NewConfigDriveMetadataService(inf.resolver, inf.platform, configDriveDiskPaths)
		err := metadataService.Load()
		if err == nil {
			return metadataService
		}
	}

	return NewHTTPMetadataService(
		"http://169.254.169.254",
		inf.resolver,
	)
}
