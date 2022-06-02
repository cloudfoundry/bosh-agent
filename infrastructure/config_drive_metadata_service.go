package infrastructure

import (
	"encoding/json"

	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type configDriveMetadataService struct {
	platform boshplatform.Platform

	diskPaths        []string
	metaDataFilePath string
	userDataFilePath string

	// Loaded state
	metaDataContents MetadataContentsType
	userDataContents UserDataContentsType

	logger boshlog.Logger
	logTag string
}

func NewConfigDriveMetadataService(
	platform boshplatform.Platform,
	diskPaths []string,
	metaDataFilePath string,
	userDataFilePath string,
	logger boshlog.Logger,
) MetadataService {
	return &configDriveMetadataService{
		platform: platform,

		diskPaths:        diskPaths,
		metaDataFilePath: metaDataFilePath,
		userDataFilePath: userDataFilePath,

		logTag: "ConfigDriveMetadataService",
		logger: logger,
	}
}

func (ms *configDriveMetadataService) GetPublicKey() (string, error) {
	if firstPublicKey, ok := ms.metaDataContents.PublicKeys["0"]; ok {
		if openSSHKey, ok := firstPublicKey["openssh-key"]; ok {
			return openSSHKey, nil
		}
	}

	return "", bosherr.Error("Failed to load openssh-key from config drive metadata service")
}

func (ms *configDriveMetadataService) GetInstanceID() (string, error) {
	if ms.metaDataContents.InstanceID == "" {
		return "", bosherr.Error("Failed to load instance-id from config drive metadata service")
	}

	ms.logger.Debug(ms.logTag, "Getting instance id: %s", ms.metaDataContents.InstanceID)
	return ms.metaDataContents.InstanceID, nil
}

func (ms *configDriveMetadataService) GetServerName() (string, error) {
	if ms.userDataContents.Server.Name == "" {
		return "", bosherr.Error("Failed to load server name from config drive metadata service")
	}

	ms.logger.Debug(ms.logTag, "Getting server name: %s", ms.userDataContents.Server.Name)
	return ms.userDataContents.Server.Name, nil
}

func (ms *configDriveMetadataService) GetNetworks() (boshsettings.Networks, error) {
	return ms.userDataContents.Networks, nil
}

func (ms *configDriveMetadataService) GetSettings() (boshsettings.Settings, error) {
	if ms.userDataContents.Settings.AgentID == "" {
		return boshsettings.Settings{}, bosherr.Error("Metadata does not provide settings")
	}
	return ms.userDataContents.Settings, nil
}

func (ms *configDriveMetadataService) IsAvailable() bool {
	if len(ms.diskPaths) == 0 {
		ms.logger.Warn(ms.logTag, "Disk paths are not given")
		return false
	}

	return ms.load() == nil
}

func (ms *configDriveMetadataService) load() error {
	ms.logger.Debug(ms.logTag, "Loading config drive metadata service")

	var err error

	for _, diskPath := range ms.diskPaths {
		err = ms.loadFromDiskPath(diskPath)
		if err == nil {
			ms.logger.Debug(ms.logTag, "Successfully loaded config from %s", diskPath)
			return nil
		}

		ms.logger.Warn(ms.logTag, "Failed to load config from %s - %s", diskPath, err.Error())
	}

	return err
}

func (ms *configDriveMetadataService) loadFromDiskPath(diskPath string) error {
	contentPaths := []string{ms.metaDataFilePath, ms.userDataFilePath}

	contents, err := ms.platform.GetFilesContentsFromDisk(diskPath, contentPaths)
	if err != nil {
		return bosherr.WrapError(err, "Reading files on config drive")
	}

	var metadata MetadataContentsType

	err = json.Unmarshal(contents[0], &metadata)
	if err != nil {
		return bosherr.WrapError(err, "Parsing config drive metadata from meta_data.json")
	}

	ms.metaDataContents = metadata

	var userdata UserDataContentsType

	err = json.Unmarshal(contents[1], &userdata)
	if err != nil {
		return bosherr.WrapError(err, "Parsing config drive metadata from user_data")
	}

	ms.userDataContents = userdata

	return nil
}
