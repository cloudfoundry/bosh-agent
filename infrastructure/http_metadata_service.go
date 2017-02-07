package infrastructure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshhttp "github.com/cloudfoundry/bosh-utils/http"
	boshhttpclient "github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type HTTPMetadataService struct {
	retryClient     boshhttpclient.HTTPClient
	metadataHost    string
	metadataHeaders map[string]string
	userdataPath    string
	instanceIDPath  string
	sshKeysPath     string
	resolver        DNSResolver
	platform        boshplat.Platform
	logTag          string
	logger          boshlog.Logger
}

func NewHTTPMetadataService(
	metadataHost string,
	metadataHeaders map[string]string,
	userdataPath string,
	instanceIDPath string,
	sshKeysPath string,
	resolver DNSResolver,
	platform boshplat.Platform,
	logger boshlog.Logger,
) DynamicMetadataService {
	return HTTPMetadataService{
		retryClient:     createRetryClient(1*time.Second, logger),
		metadataHost:    metadataHost,
		metadataHeaders: metadataHeaders,
		userdataPath:    userdataPath,
		instanceIDPath:  instanceIDPath,
		sshKeysPath:     sshKeysPath,
		resolver:        resolver,
		platform:        platform,
		logTag:          "httpMetadataService",
		logger:          logger,
	}
}

func NewHTTPMetadataServiceWithCustomRetryDelay(
	metadataHost string,
	metadataHeaders map[string]string,
	userdataPath string,
	instanceIDPath string,
	sshKeysPath string,
	resolver DNSResolver,
	platform boshplat.Platform,
	logger boshlog.Logger,
	retryDelay time.Duration,
) DynamicMetadataService {
	return HTTPMetadataService{
		retryClient:     createRetryClient(retryDelay, logger),
		metadataHost:    metadataHost,
		metadataHeaders: metadataHeaders,
		userdataPath:    userdataPath,
		instanceIDPath:  instanceIDPath,
		sshKeysPath:     sshKeysPath,
		resolver:        resolver,
		platform:        platform,
		logTag:          "httpMetadataService",
		logger:          logger,
	}
}

func (ms HTTPMetadataService) Load() error {
	return nil
}

func (ms HTTPMetadataService) GetPublicKey() (string, error) {
	if ms.sshKeysPath == "" {
		return "", nil
	}

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s%s", ms.metadataHost, ms.sshKeysPath)
	resp, err := ms.retryClient.GetCustomized(url, ms.addHeaders())
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Getting open ssh key from url %s", url)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			ms.logger.Warn(ms.logTag, "Failed to close response body when getting ssh key: %s", err.Error())
		}
	}()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", bosherr.WrapError(err, "Reading ssh key response body")
	}

	return string(bytes), nil
}

func (ms HTTPMetadataService) GetInstanceID() (string, error) {
	if ms.instanceIDPath == "" {
		return "", nil
	}

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s%s", ms.metadataHost, ms.instanceIDPath)
	resp, err := ms.retryClient.GetCustomized(url, ms.addHeaders())
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Getting instance id from url %s", url)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			ms.logger.Warn(ms.logTag, "Failed to close response body when getting instance id: %s", err.Error())
		}
	}()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", bosherr.WrapError(err, "Reading instance id response body")
	}

	return string(bytes), nil
}

func (ms HTTPMetadataService) GetValueAtPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("Can not retrieve metadata value for empthy path")
	}

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s%s", ms.metadataHost, path)
	resp, err := ms.retryClient.GetCustomized(url, ms.addHeaders())
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Getting value from url %s", url)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			ms.logger.Warn(ms.logTag, "Failed to close response body when getting value from path: %s", err.Error())
		}
	}()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", bosherr.WrapError(err, fmt.Sprintf("Reading response body from %s", url))
	}

	return string(bytes), nil
}
func (ms HTTPMetadataService) GetServerName() (string, error) {
	userData, err := ms.getUserData()
	if err != nil {
		return "", bosherr.WrapError(err, "Getting user data")
	}

	serverName := userData.Server.Name

	if len(serverName) == 0 {
		return "", bosherr.Error("Empty server name")
	}

	return serverName, nil
}

func (ms HTTPMetadataService) GetRegistryEndpoint() (string, error) {
	userData, err := ms.getUserData()
	if err != nil {
		return "", bosherr.WrapError(err, "Getting user data")
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

func (ms HTTPMetadataService) GetNetworks() (boshsettings.Networks, error) {
	return nil, nil
}

func (ms HTTPMetadataService) IsAvailable() bool { return true }

func (ms HTTPMetadataService) getUserData() (UserDataContentsType, error) {
	var userData UserDataContentsType

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return userData, err
	}

	userDataURL := fmt.Sprintf("%s%s", ms.metadataHost, ms.userdataPath)
	userDataResp, err := ms.retryClient.GetCustomized(userDataURL, ms.addHeaders())
	if err != nil {
		return userData, bosherr.WrapErrorf(err, "Getting user data from url %s", userDataURL)
	}

	defer func() {
		if err := userDataResp.Body.Close(); err != nil {
			ms.logger.Warn(ms.logTag, "Failed to close response body when getting user data: %s", err.Error())
		}
	}()

	userDataBytes, err := ioutil.ReadAll(userDataResp.Body)
	if err != nil {
		return userData, bosherr.WrapError(err, "Reading user data response body")
	}

	err = json.Unmarshal(userDataBytes, &userData)
	if err != nil {
		return userData, bosherr.WrapErrorf(err, "Unmarshalling user data '%s'", string(userDataBytes))
	}

	return userData, nil
}

func (ms HTTPMetadataService) ensureMinimalNetworkSetup() error {
	// We check for configuration presence instead of verifying
	// that network is reachable because we want to preserve
	// network configuration that was passed to agent.
	configuredInterfaces, err := ms.platform.GetConfiguredNetworkInterfaces()
	if err != nil {
		return bosherr.WrapError(err, "Getting configured network interfaces")
	}

	if len(configuredInterfaces) == 0 {
		ms.logger.Debug(ms.logTag, "No configured networks found, setting up DHCP network")
		err = ms.platform.SetupNetworking(boshsettings.Networks{
			"eth0": {
				Type: boshsettings.NetworkTypeDynamic,
			},
		})
		if err != nil {
			return bosherr.WrapError(err, "Setting up initial DHCP network")
		}
	}

	return nil
}

func (ms HTTPMetadataService) addHeaders() func(*http.Request) {
	return func(req *http.Request) {
		for key, value := range ms.metadataHeaders {
			req.Header.Add(key, value)
		}
	}
}

func createRetryClient(delay time.Duration, logger boshlog.Logger) boshhttpclient.HTTPClient {
	return boshhttpclient.NewHTTPClient(
		boshhttp.NewRetryClient(
			boshhttpclient.CreateDefaultClientInsecureSkipVerify(), 10, delay, logger),
		logger)
}
