package downloader

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/installer"
	"github.com/cloudfoundry/bosh-agent/errors"
	"github.com/cloudfoundry/bosh-agent/logger"
)

type Downloader struct {
	config    auth.SSLConfig
	installer installer.Installer
}

func NewDownloader(config auth.SSLConfig, installer installer.Installer) *Downloader {
	return &Downloader{
		config:    config,
		installer: installer,
	}
}

func (d *Downloader) httpClient() (*http.Client, error) {
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(([]byte)(d.config.CACertPem)) {
		return nil, errors.Error("Failed to load CA cert")
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    certPool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		},
	}

	return &http.Client{Transport: tr}, nil
}

func (d *Downloader) Download(logger logger.Logger, url string) error {
	logger.Info("Download", "Downloading %s...", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("Download", "Couldn't make the request to %s: %s", url, err.Error())
		return err
	}

	client, err := d.httpClient()
	if err != nil {
		logger.Error("Download", "Couldn't make the http client %s", err.Error())
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Download", "Couldn't do the request (%s): %s", req, err.Error())
		return err
	}

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("Download failed, bad response: %s", resp.Status)
		logger.Error("Download", err.Error())
		return err
	}

	certificateVerifier := auth.CertificateVerifier{AllowedNames: d.config.PkixNames}

	err = certificateVerifier.Verify(resp.TLS.PeerCertificates)
	if err != nil {
		return err
	}

	logger.Info("Download", "Downloading complete. Installing...")
	err = d.installer.Install(resp.Body)
	if err != nil {
		return err
	}

	logger.Info("Download", "Download succeeded")
	return nil
}
