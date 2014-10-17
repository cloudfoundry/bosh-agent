package infrastructure

import (
	"strings"

	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type RegistryProvider interface {
	GetRegistry() Registry
}

type registryProvider struct {
	metadataService          MetadataService
	fallbackFileRegistryPath string
	fs                       boshsys.FileSystem
}

func NewRegistryProvider(
	metadataService MetadataService,
	fallbackFileRegistryPath string,
	fs boshsys.FileSystem,
) RegistryProvider {
	return &registryProvider{
		metadataService:          metadataService,
		fallbackFileRegistryPath: fallbackFileRegistryPath,
		fs: fs,
	}
}

func (p *registryProvider) GetRegistry() Registry {
	registryEndpoint, err := p.metadataService.GetRegistryEndpoint()
	if err != nil {
		return NewFileRegistry(p.fallbackFileRegistryPath, p.fs)
	}

	if strings.HasPrefix(registryEndpoint, "http") {
		return NewHTTPRegistry(p.metadataService, false)
	}

	return NewFileRegistry(registryEndpoint, p.fs)
}
