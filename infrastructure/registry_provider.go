package infrastructure

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type RegistryProvider interface {
	GetRegistry() (Registry, error)
}

type registryProvider struct {
	metadataService MetadataService
	useServerName   bool
	platform        boshplat.Platform
	fs              boshsys.FileSystem
	logTag          string
	logger          boshlog.Logger
}

func NewRegistryProvider(
	metadataService MetadataService,
	platform boshplat.Platform,
	useServerName bool,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) RegistryProvider {
	return &registryProvider{
		metadataService: metadataService,
		platform:        platform,
		useServerName:   useServerName,
		fs:              fs,
		logTag:          "registryProvider",
		logger:          logger,
	}
}

func (p *registryProvider) GetRegistry() (Registry, error) {
	registryEndpoint, err := p.metadataService.GetRegistryEndpoint()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting registry endpoint")
	}

	if strings.HasPrefix(registryEndpoint, "http") {
		p.logger.Debug(p.logTag, "Using http registry at %s", registryEndpoint)
		return NewHTTPRegistry(p.metadataService, p.platform, p.useServerName), nil
	}

	p.logger.Debug(p.logTag, "Using file registry at %s", registryEndpoint)
	return NewFileRegistry(registryEndpoint, p.fs), nil
}
