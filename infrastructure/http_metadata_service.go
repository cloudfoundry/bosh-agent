package infrastructure

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshhttp "github.com/cloudfoundry/bosh-utils/http"
	boshhttpclient "github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type HTTPMetadataService struct {
	client          boshhttpclient.HTTPClient
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
		client:          createRetryClient(1*time.Second, logger),
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
		client:          createRetryClient(retryDelay, logger),
		metadataHost:    metadataHost,
		metadataHeaders: metadataHeaders,
		userdataPath:    userdataPath,
		instanceIDPath:  instanceIDPath,
		sshKeysPath:     sshKeysPath,
		resolver:        resolver,
		platform:        platform,

		logTag: "httpMetadataService",
		logger: logger,
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

	respBytes, err := ms.fetchBytes(fmt.Sprintf("%s%s", ms.metadataHost, ms.sshKeysPath))
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Getting open SSH key")
	}

	return string(respBytes), nil
}

func (ms HTTPMetadataService) GetInstanceID() (string, error) {
	if ms.instanceIDPath == "" {
		return "", nil
	}

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return "", err
	}

	respBytes, err := ms.fetchBytes(fmt.Sprintf("%s%s", ms.metadataHost, ms.instanceIDPath))
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Getting instance id")
	}

	return string(respBytes), nil
}

func (ms HTTPMetadataService) GetValueAtPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("Can not retrieve metadata value for empthy path")
	}

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return "", err
	}

	respBytes, err := ms.fetchBytes(fmt.Sprintf("%s%s", ms.metadataHost, path))
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Getting value at path '%s'", path)
	}

	return string(respBytes), nil
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
	userData, err := ms.getUserData()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting user data")
	}

	return userData.Networks, nil
}

func (ms HTTPMetadataService) IsAvailable() bool { return true }

func (ms HTTPMetadataService) getUserData() (UserDataContentsType, error) {
	var userData UserDataContentsType

	err := ms.ensureMinimalNetworkSetup()
	if err != nil {
		return userData, err
	}

	respBytes, err := ms.fetchBytes(fmt.Sprintf("%s%s", ms.metadataHost, ms.userdataPath))
	if err != nil {
		return userData, bosherr.WrapErrorf(err, "Getting user data")
	}

	err = json.Unmarshal(respBytes, &userData)
	if err != nil {
		respBytesWithoutQuotes := strings.Replace(string(respBytes), `"`, ``, -1)

		decodedUserData, err := base64.RawURLEncoding.DecodeString(respBytesWithoutQuotes)
		if err != nil {
			return userData, bosherr.WrapError(err, "Decoding url encoded user data")
		}

		err = json.Unmarshal([]byte(decodedUserData), &userData)
		if err != nil {
			return userData, bosherr.WrapErrorf(err, "Unmarshalling url decoded user data '%s'", decodedUserData)
		}
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

func (ms HTTPMetadataService) fetchBytes(url string) ([]byte, error) {
	// todo use proper fs url?
	cachePath := fmt.Sprintf("/var/vcap/bosh/http-metadata-service-%x", sha1.Sum([]byte(url)))

	cachedRespBytes, err := ms.platform.GetFs().ReadFile(cachePath)
	if err == nil {
		return cachedRespBytes, nil
	}

	addHeaders := func(req *http.Request) {
		for key, value := range ms.metadataHeaders {
			req.Header.Add(key, value)
		}
	}

	resp, err := ms.client.GetCustomized(url, addHeaders)
	if err != nil {
		return nil, bosherr.WrapErrorf(err, "Getting user data over URL '%s'", url)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			ms.logger.Warn(ms.logTag, "Failed to close response body when getting user data: %s", err.Error())
		}
	}()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, bosherr.WrapError(err, "Reading user data response body")
	}

	err = ms.platform.GetFs().WriteFile(cachePath, respBytes)
	if err != nil {
		return nil, bosherr.WrapError(err, "Caching response body")
	}

	return respBytes, nil
}

func createRetryClient(delay time.Duration, logger boshlog.Logger) boshhttpclient.HTTPClient {
	return boshhttpclient.NewHTTPClient(
		boshhttp.NewRetryClient(
			boshhttpclient.CreateDefaultClient(nil), 10, delay, logger),
		logger)
}
