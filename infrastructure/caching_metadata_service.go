package infrastructure

import (
	"encoding/json"

	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type CachingMetadataService struct {
	userdataPath      string
	instanceIDPath    string
	sshKeysPath       string
	userdataCachePath string
	resolver          DNSResolver
	platform          boshplat.Platform
	fs                boshsys.FileSystem
	logTag            string
	logger            boshlog.Logger

	HTTPMetadataService
}

func NewCachingMetadataService(
	userdataCachePath string,
	resolver DNSResolver,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
	httpMetadataService HTTPMetadataService,
) DynamicMetadataService {
	return CachingMetadataService{
		userdataCachePath:   userdataCachePath,
		resolver:            resolver,
		fs:                  fs,
		logTag:              "cachingMetadataService",
		logger:              logger,
		HTTPMetadataService: httpMetadataService,
	}
}

func (ms CachingMetadataService) GetServerName() (string, error) {
	userData, err := ms.getUserDataFromCache()
	if err != nil {
		ms.logger.Warn(ms.logTag, "Failed to get user data from local cache file: %s", err.Error())
		userData, err = ms.getUserData()
		if err != nil {
			return "", bosherr.WrapError(err, "Getting user data from remote server")
		}
		err = ms.cacheUserData(userData)
		if err != nil {
			return "", bosherr.WrapError(err, "Caching user data")
		}
	}

	serverName := userData.Server.Name

	if len(serverName) == 0 {
		return "", bosherr.Error("Empty server name")
	}

	return serverName, nil
}

func (ms CachingMetadataService) GetRegistryEndpoint() (string, error) {
	userData, err := ms.getUserDataFromCache()
	if err != nil {
		ms.logger.Warn(ms.logTag, "Failed to get registry endpoint from local cache file: %s", err.Error())
		userData, err = ms.getUserData()
		if err != nil {
			return "", bosherr.WrapError(err, "Getting user data from remote server")
		}
		err = ms.cacheUserData(userData)
		if err != nil {
			return "", bosherr.WrapError(err, "Caching user data")
		}
	}

	endpoint := userData.Registry.Endpoint
	nameServers := userData.DNS.Nameserver

	if len(nameServers) > 0 {
		endpoint, err = ms.resolver.LookupHost(nameServers, endpoint)
		if err != nil {
			return "", bosherr.WrapError(err, "Resolving registry endpoint")
		}
	}

	return endpoint, nil
}

func (ms CachingMetadataService) getUserDataFromCache() (UserDataContentsType, error) {
	var userData UserDataContentsType
	var userDataBytes []byte
	var err error

	defer func() {
		if err != nil {
			ms.logger.Debug(ms.logTag, "Cleaning up local cache file")
			err := ms.fs.RemoveAll(ms.userdataCachePath)
			if err != nil {
				ms.logger.Warn(ms.logTag, "Failed to clean up local cache file")
			}
		}
	}()

	userDataBytes, err = ms.fs.ReadFile(ms.userdataCachePath)
	if err != nil {
		return UserDataContentsType{}, bosherr.WrapErrorf(err, "Getting user data from local cache file %s", ms.userdataCachePath)
	}

	err = json.Unmarshal(userDataBytes, &userData)
	if err != nil {
		return userData, bosherr.WrapErrorf(err, "Unmarshalling user data '%s'", userDataBytes)
	}
	return userData, nil
}

func (ms CachingMetadataService) cacheUserData(userDataContents UserDataContentsType) error {
	userDataBytes, err := json.Marshal(userDataContents)
	err = ms.fs.WriteFile(ms.userdataCachePath, userDataBytes)
	if err != nil {
		return bosherr.WrapErrorf(err, "Generating user data cache file %s", ms.userdataCachePath)
	}

	return nil
}
