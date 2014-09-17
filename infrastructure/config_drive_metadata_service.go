package infrastructure

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

type configDriveMetadataService struct {
	metadataContents MetadataContentsType
	userdataContents UserDataContentsType
	resolver         DNSResolver
	platform         boshplatform.Platform
	diskPaths        []string
	metadataFilePath string
	userdataFilePath string
}

func NewConfigDriveMetadataService(
	resolver DNSResolver,
	platform boshplatform.Platform,
	diskPaths []string,
	metadataFilePath string,
	userdataFilePath string,
) *configDriveMetadataService {
	return &configDriveMetadataService{
		resolver:         resolver,
		platform:         platform,
		diskPaths:        diskPaths,
		metadataFilePath: metadataFilePath,
		userdataFilePath: userdataFilePath,
	}
}

func (ms *configDriveMetadataService) Load() error {
	var err error

	for _, diskPath := range ms.diskPaths {
		err = ms.loadFromDiskPath(diskPath)
		if err == nil {
			return nil
		}
	}

	return err
}

func (ms *configDriveMetadataService) GetPublicKey() (string, error) {
	if firstPublicKey, ok := ms.metadataContents.PublicKeys["0"]; ok {
		if openSSHKey, ok := firstPublicKey["openssh-key"]; ok {
			return openSSHKey, nil
		}
	}

	return "", bosherr.New("Failed to load openssh-key from config drive metadata service")
}

func (ms *configDriveMetadataService) GetInstanceID() (string, error) {
	if ms.metadataContents.InstanceID != "" {
		return ms.metadataContents.InstanceID, nil
	}

	return "", bosherr.New("Failed to load instance-id from config drive metadata service")
}

func (ms *configDriveMetadataService) GetServerName() (string, error) {
	if ms.userdataContents.Server.Name == "" {
		return "", bosherr.New("Failed to load server name from config drive metadata service")
	}

	return ms.userdataContents.Server.Name, nil
}

func (ms *configDriveMetadataService) GetRegistryEndpoint() (string, error) {
	if ms.userdataContents.Registry.Endpoint == "" {
		return "", bosherr.New("Failed to load registry endpoint from config drive metadata service")

	}

	endpoint := ms.userdataContents.Registry.Endpoint
	nameServers := ms.userdataContents.DNS.Nameserver

	if len(nameServers) > 0 {
		var err error
		endpoint, err = ms.resolver.LookupHost(nameServers, endpoint)
		if err != nil {
			return "", bosherr.WrapError(err, "Resolving registry endpoint")
		}
	}

	return endpoint, nil
}

func (ms *configDriveMetadataService) loadFromDiskPath(diskPath string) error {
	contents, err := ms.platform.GetFileContentsFromDisk(diskPath, ms.metadataFilePath)
	if err != nil {
		return bosherr.WrapError(err, "Reading contents of meta_data.json on config drive")
	}

	var metadata MetadataContentsType
	err = json.Unmarshal(contents, &metadata)
	if err != nil {
		return bosherr.WrapError(err, "Parsing config drive metadata from meta_data.json")
	}
	ms.metadataContents = metadata

	contents, err = ms.platform.GetFileContentsFromDisk(diskPath, ms.userdataFilePath)
	if err != nil {
		return bosherr.WrapError(err, "Reading contents of user_data on config drive")
	}

	var userdata UserDataContentsType
	err = json.Unmarshal(contents, &userdata)
	if err != nil {
		return bosherr.WrapError(err, "Parsing config drive metadata from user_data")
	}
	ms.userdataContents = userdata

	return nil
}
