package monit

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data" // translations between char sets

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshhttp "github.com/cloudfoundry/bosh-agent/http"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
)

type httpClient struct {
	client                 boshhttp.Client
	host                   string
	username               string
	password               string
	defaultRetryDelay      time.Duration
	stopRetryDelay         time.Duration
	unmonitorRetryDelay    time.Duration
	defaultRetryAttempts   int
	stopRetryAttempts      int
	unmonitorRetryAttempts int
	logger                 boshlog.Logger
	timeService            boshtime.Service
}

func NewHTTPClient(
	host, username, password string,
	client boshhttp.Client,
	defaultRetryDelay time.Duration,
	stopRetryDelay time.Duration,
	unmonitorRetryDelay time.Duration,
	defaultRetryAttempts int,
	stopRetryAttempts int,
	unmonitorRetryAttempts int,
	logger boshlog.Logger,
	timeService boshtime.Service,
) httpClient {
	return httpClient{
		host:                   host,
		username:               username,
		password:               password,
		client:                 client,
		defaultRetryDelay:      defaultRetryDelay,
		stopRetryDelay:         stopRetryDelay,
		unmonitorRetryDelay:    unmonitorRetryDelay,
		defaultRetryAttempts:   defaultRetryAttempts,
		stopRetryAttempts:      stopRetryAttempts,
		unmonitorRetryAttempts: unmonitorRetryAttempts,
		logger:                 logger,
		timeService:            timeService,
	}
}

func (c httpClient) ServicesInGroup(name string) (services []string, err error) {
	status, err := c.status()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting status from Monit")
	}

	serviceGroup, found := status.ServiceGroups.Get(name)
	if !found {
		return []string{}, nil
	}

	return serviceGroup.Services, nil
}

func (c httpClient) StartService(serviceName string) (err error) {
	response, err := c.makeRequest(c.monitURL(serviceName), "POST", "action=start", c.defaultRetryDelay, c.defaultRetryAttempts)
	if err != nil {
		return bosherr.WrapError(err, "Sending start request to monit")
	}

	defer response.Body.Close()

	err = c.validateResponse(response)
	if err != nil {
		return bosherr.WrapError(err, "Starting Monit service %s", serviceName)
	}

	return nil
}

func (c httpClient) StopService(serviceName string) error {
	c.logger.Debug("http-client", "Stop retry delay is %d", c.stopRetryDelay)
	response, err := c.makeRequest(c.monitURL(serviceName), "POST", "action=stop", c.stopRetryDelay, c.stopRetryAttempts)
	if err != nil {
		return bosherr.WrapError(err, "Sending stop request to monit")
	}

	defer response.Body.Close()

	err = c.validateResponse(response)
	if err != nil {
		return bosherr.WrapError(err, "Stopping Monit service %s", serviceName)
	}

	return nil
}

func (c httpClient) UnmonitorService(serviceName string) error {
	c.logger.Debug("http-client", "Unmonitor retry delay is %d", c.unmonitorRetryDelay)
	response, err := c.makeRequest(c.monitURL(serviceName), "POST", "action=unmonitor", c.unmonitorRetryDelay, c.unmonitorRetryAttempts)
	if err != nil {
		return bosherr.WrapError(err, "Sending unmonitor request to monit")
	}

	defer response.Body.Close()

	err = c.validateResponse(response)
	if err != nil {
		return bosherr.WrapError(err, "Unmonitoring Monit service %s", serviceName)
	}

	return nil
}

func (c httpClient) Status() (Status, error) {
	return c.status()
}

func (c httpClient) status() (status, error) {
	c.logger.Debug("http-client", "status function called")
	url := c.monitURL("/_status2")
	url.RawQuery = "format=xml"

	response, err := c.makeRequest(url, "GET", "", c.defaultRetryDelay, c.defaultRetryAttempts)
	if err != nil {
		return status{}, bosherr.WrapError(err, "Sending status request to monit")
	}

	defer response.Body.Close()

	err = c.validateResponse(response)
	if err != nil {
		return status{}, bosherr.WrapError(err, "Getting monit status")
	}

	decoder := xml.NewDecoder(response.Body)
	decoder.CharsetReader = charset.NewReader

	var st status

	err = decoder.Decode(&st)
	if err != nil {
		return status{}, bosherr.WrapError(err, "Unmarshalling Monit status")
	}

	return st, nil
}

func (c httpClient) monitURL(thing string) url.URL {
	return url.URL{
		Scheme: "http",
		Host:   c.host,
		Path:   path.Join("/", thing),
	}
}

func (c httpClient) validateResponse(response *http.Response) error {
	if response.StatusCode == http.StatusOK {
		return nil
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return bosherr.WrapError(err, "Reading body of failed Monit response")
	}

	c.logger.Debug("http-client", "Request failed with %s: %s", response.Status, string(body))

	return bosherr.New("Request failed with %s: %s", response.Status, string(body))
}

func (c httpClient) makeRequest(url url.URL, method, requestBody string, unavailableDelay time.Duration, retryAttempts int) (response *http.Response, err error) {
	c.logger.Debug("http-client", "makeRequest with url %s", url.String())

	canReset := true

	for attempt := 0; attempt < retryAttempts; attempt++ {
		c.logger.Debug("http-client", "Retrying %d", attempt)

		if response != nil {
			response.Body.Close()
		}

		delay := c.defaultRetryDelay
		response, err = c.doRequest(url, method, requestBody)
		if err != nil {
			c.logger.Debug("http-client", "Got err %v", err)
		} else {
			c.logger.Debug("http-client", "Got response with status %d", response.StatusCode)

			if response.StatusCode == 200 {
				return
			}

			if response.StatusCode == 503 && canReset {
				delay = unavailableDelay
			}

			if response.StatusCode != 503 && canReset {
				attempt = 0
				canReset = false
			}
		}

		c.logger.Debug("http-client", "Going to sleep for %d", delay)
		c.timeService.Sleep(delay)
	}

	return
}

func (c httpClient) doRequest(url url.URL, method, requestBody string) (response *http.Response, err error) {
	var request *http.Request

	request, err = http.NewRequest(method, url.String(), strings.NewReader(requestBody))
	if err != nil {
		return
	}

	request.SetBasicAuth(c.username, c.password)

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err = c.client.Do(request)

	return
}
