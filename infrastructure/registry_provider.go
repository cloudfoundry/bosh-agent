package infrastructure

import (
	"strings"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type RegistryProvider interface {
	GetRegistry() Registry
}

type registryProvider struct {
	metadataService          MetadataService
	fallbackFileRegistryPath string
	fs                       boshsys.FileSystem
	logger                   boshlog.Logger
	logTag                   string
}

func NewRegistryProvider(
	metadataService MetadataService,
	fallbackFileRegistryPath string,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) RegistryProvider {
	return &registryProvider{
		metadataService:          metadataService,
		fallbackFileRegistryPath: fallbackFileRegistryPath,
		fs:     fs,
		logger: logger,
		logTag: "registryProvider",
	}
}

func (p *registryProvider) GetRegistry() Registry {
	registryEndpoint, err := p.metadataService.GetRegistryEndpoint()
	if err != nil {
		p.logger.Debug(p.logTag, "Using fallback file registry %s", p.fallbackFileRegistryPath)
		return NewFileRegistry(p.fallbackFileRegistryPath, p.fs)
	}

	if strings.HasPrefix(registryEndpoint, "http") {
		p.logger.Debug(p.logTag, "Using http registry at %s", registryEndpoint)
		return NewHTTPRegistry(p.metadataService, false)
	}

	p.logger.Debug(p.logTag, "Using file registry at %s", registryEndpoint)
	return NewFileRegistry(registryEndpoint, p.fs)
}
