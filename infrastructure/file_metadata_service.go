package infrastructure

import (
	"encoding/json"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type PublicKeyContent struct {
	PublicKey string `json:"public_key"`
}

type fileMetadataService struct {
	metaDataFilePath string
	userDataFilePath string
	settingsFilePath string
	fs               boshsys.FileSystem

	logger boshlog.Logger
	logTag string
}

func NewFileMetadataService(
	metaDataFilePath string,
	userDataFilePath string,
	settingsFilePath string,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) MetadataService {
	return fileMetadataService{
		metaDataFilePath: metaDataFilePath,
		userDataFilePath: userDataFilePath,
		settingsFilePath: settingsFilePath,
		fs:               fs,
		logTag:           "fileMetadataService",
		logger:           logger,
	}
}

func (ms fileMetadataService) Load() error {
	return nil
}

func (ms fileMetadataService) GetPublicKey() (string, error) {
	var p PublicKeyContent

	err := ms.unmarshallFile(ms.metaDataFilePath, &p)
	if err != nil {
		return "", err
	}
	return p.PublicKey, nil
}

func (ms fileMetadataService) GetInstanceID() (string, error) {
	var metadata MetadataContentsType

	err := ms.unmarshallFile(ms.metaDataFilePath, &metadata)
	if err != nil {
		return "", err
	}
	return metadata.InstanceID, nil
}

func (ms fileMetadataService) GetServerName() (string, error) {
	var userData UserDataContentsType

	err := ms.unmarshallFile(ms.userDataFilePath, &userData)
	if err != nil {
		return "", err
	}
	return userData.Server.Name, nil
}

func (ms fileMetadataService) GetRegistryEndpoint() (string, error) {
	var userData UserDataContentsType

	contents, err := ms.fs.ReadFile(ms.userDataFilePath)
	if err != nil {
		// Older versions of bosh-warden-cpi placed
		// full settings file at a specific location.
		return ms.settingsFilePath, nil
	}

	err = json.Unmarshal([]byte(contents), &userData)
	if err != nil {
		return "", bosherr.WrapError(err, "Unmarshalling user data")
	}

	ms.logger.Debug(ms.logTag, "Read user data '%#v'", userData)
	return userData.Registry.Endpoint, nil
}

func (ms fileMetadataService) GetNetworks() (boshsettings.Networks, error) {
	var userData UserDataContentsType

	err := ms.unmarshallFile(ms.userDataFilePath, &userData)
	if err != nil {
		return nil, err
	}
	return userData.Networks, nil
}

func (ms fileMetadataService) unmarshallFile(filePath string, data interface{}) error {

	contents, err := ms.fs.ReadFile(filePath)
	if err != nil {
		return bosherr.WrapError(err, "Reading user data: File not found ")
	}

	err = json.Unmarshal([]byte(contents), data)
	if err != nil {
		return bosherr.WrapError(err, "Unmarshalling user data")
	}

	ms.logger.Debug(ms.logTag, "Read user data '%#v'", data)
	return nil
}

func (ms fileMetadataService) IsAvailable() bool {
	return ms.fs.FileExists(ms.settingsFilePath)
}
