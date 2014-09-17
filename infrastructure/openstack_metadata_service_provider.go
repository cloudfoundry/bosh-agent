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
	if inf.metadataServiceOptions.UseConfigDrive {
		inf.logger.Debug(inf.logTag, "Loading config drive metadata service")
		configDriveDiskPaths := []string{
			"/dev/disk/by-label/CONFIG-2",
			"/dev/disk/by-label/config-2",
		}

		metadataService := NewConfigDriveMetadataService(
			inf.resolver,
			inf.platform,
			configDriveDiskPaths,
			"ec2/latest/meta-data.json",
			"ec2/latest/user-data",
			inf.logger,
		)
		err := metadataService.Load()
		if err == nil {
			return metadataService
		}

		inf.logger.Warn(inf.logTag, "Failed to load config drive metadata service", err)
	}

	inf.logger.Debug(inf.logTag, "Using http metadata service")
	return NewHTTPMetadataService(
		"http://169.254.169.254",
		inf.resolver,
	)
}
