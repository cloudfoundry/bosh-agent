package infrastructure

import (
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

type openstackServiceProvider struct {
	resolver               DNSResolver
	platform               boshplatform.Platform
	metadataServiceOptions MetadataServiceOptions
	logger                 boshlog.Logger
	logTag                 string
}

func NewOpenstackMetadataServiceProvider(
	resolver DNSResolver,
	platform boshplatform.Platform,
	options MetadataServiceOptions,
	logger boshlog.Logger,
) openstackServiceProvider {
	return openstackServiceProvider{
		resolver:               resolver,
		platform:               platform,
		metadataServiceOptions: options,
		logger:                 logger,
		logTag:                 "OpenstackMetadataServiceProvider",
	}
}

func (inf openstackServiceProvider) Get() MetadataService {
	httpMetadataService := NewHTTPMetadataService(
		"http://169.254.169.254",
		inf.resolver,
	)

	if inf.metadataServiceOptions.UseConfigDrive {
		configDriveDiskPaths := []string{
			"/dev/disk/by-label/CONFIG-2",
			"/dev/disk/by-label/config-2",
		}

		confDriveMetadataService := NewConfigDriveMetadataService(
			inf.resolver,
			inf.platform,
			configDriveDiskPaths,
			"ec2/latest/meta-data.json",
			"ec2/latest/user-data",
			inf.logger,
		)

		metadataService := NewMultiSourceMetadataService(
			confDriveMetadataService,
			httpMetadataService,
		)

		return metadataService
	}

	return httpMetadataService
}
