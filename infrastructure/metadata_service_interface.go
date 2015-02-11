package infrastructure

type MetadataService interface {
	IsAvailable() bool
	GetPublicKey() (string, error)
	GetInstanceID() (string, error)
	GetServerName() (string, error)
	GetRegistryEndpoint() (string, error)
}

type MetadataServiceOptions struct {
	UseConfigDrive bool
}

type MetadataServiceProvider interface {
	Get() MetadataService
}

type UserDataContentsType struct {
	Registry struct {
		Endpoint string
	}
	Server struct {
		Name string // Name given by CPI e.g. vm-384sd4-r7re9e...
	}
	DNS struct {
		Nameserver []string
	}
}
