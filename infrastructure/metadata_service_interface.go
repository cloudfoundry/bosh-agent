package infrastructure

type MetadataService interface {
	GetPublicKey() (string, error)
	GetInstanceID() (string, error)
	GetServerName() (string, error)
	GetRegistryEndpoint() (string, error)
}

type MetadataServiceOptions struct {
	UseConfigDrive bool
}

type MetadataServiceProvider interface {
	GetMetadataService() MetadataService
}
