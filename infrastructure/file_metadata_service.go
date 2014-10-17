package infrastructure

import (
	"encoding/json"
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type fileMetadataService struct {
	userDataFilePath string
	fs               boshsys.FileSystem
	logger           boshlog.Logger
	logTag           string
}

func NewFileMetadataService(
	userDataFilePath string,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) fileMetadataService {
	return fileMetadataService{
		userDataFilePath: userDataFilePath,
		fs:               fs,
		logger:           logger,
		logTag:           "fileMetadataService",
	}
}

func (ms fileMetadataService) Load() error {
	return nil
}

func (ms fileMetadataService) GetPublicKey() (string, error) {
	return "", nil
}

func (ms fileMetadataService) GetInstanceID() (string, error) {
	return "", nil
}

func (ms fileMetadataService) GetServerName() (string, error) {
	return "", nil
}

func (ms fileMetadataService) GetRegistryEndpoint() (string, error) {
	var userData UserDataContentsType

	contents, err := ms.fs.ReadFile(ms.userDataFilePath)
	if err != nil {
		return "", bosherr.WrapError(err, "Reading user data file")
	}

	err = json.Unmarshal([]byte(contents), &userData)
	if err != nil {
		return "", bosherr.WrapError(err, "Unmarshalling user data")
	}

	ms.logger.Debug(ms.logTag, "Read user data %#v", userData)

	return userData.Registry.Endpoint, nil
}

func (ms fileMetadataService) IsAvailable() bool {
	return true
}
