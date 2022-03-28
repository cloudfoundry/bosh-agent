package httpblobprovider

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type HTTPBlobImpl struct {
	fs               boshsys.FileSystem
	createAlgorithms []boshcrypto.Algorithm
	httpClient       *http.Client
}

func NewHTTPBlobImpl(fs boshsys.FileSystem, httpClient *http.Client) *HTTPBlobImpl {
	var DefaultCryptoAlgorithms = []boshcrypto.Algorithm{boshcrypto.DigestAlgorithmSHA1, boshcrypto.DigestAlgorithmSHA512}

	return NewHTTPBlobImplWithDigestAlgorithms(fs, httpClient, DefaultCryptoAlgorithms)
}

func NewHTTPBlobImplWithDigestAlgorithms(fs boshsys.FileSystem, httpClient *http.Client, algorithms []boshcrypto.Algorithm) *HTTPBlobImpl {
	return &HTTPBlobImpl{
		fs:               fs,
		createAlgorithms: algorithms,
		httpClient:       httpClient,
	}
}

func (h *HTTPBlobImpl) Upload(signedURL, filepath string, headers map[string]string) (boshcrypto.MultipleDigest, error) {
	digest, err := boshcrypto.NewMultipleDigestFromPath(filepath, h.fs, h.createAlgorithms)
	if err != nil {
		return boshcrypto.MultipleDigest{}, err
	}

	// Do not close the file in the happy path because the client.Do will handle that.
	file, err := h.fs.OpenFile(filepath, os.O_RDONLY, 0)
	if err != nil {
		return boshcrypto.MultipleDigest{}, err
	}

	stat, err := h.fs.Stat(filepath)
	if err != nil {
		defer file.Close()
		return boshcrypto.MultipleDigest{}, err
	}

	req, err := http.NewRequest("PUT", signedURL, file) //nolint:noctx
	if err != nil {
		defer file.Close()
		return boshcrypto.MultipleDigest{}, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Expect", "100-continue")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	req.ContentLength = stat.Size()

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return boshcrypto.MultipleDigest{}, err
	}
	if !isSuccess(resp) {
		return boshcrypto.MultipleDigest{}, fmt.Errorf("Error executing PUT for %s, response was %d", file.Name(), resp.StatusCode)
	}

	return digest, nil
}

func (h *HTTPBlobImpl) Get(signedURL string, digest boshcrypto.Digest, headers map[string]string) (string, error) {
	file, err := h.fs.TempFile("bosh-http-blob-provider-GET")
	if err != nil {
		return "", bosherr.WrapError(err, "Creating temporary file")
	}

	req, err := http.NewRequest("GET", signedURL, strings.NewReader("")) //nolint:noctx
	if err != nil {
		defer file.Close()
		return "", bosherr.WrapError(err, "Creating Get Request")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return file.Name(), bosherr.WrapError(err, "Excuting GET request")
	}

	if !isSuccess(resp) {
		return file.Name(), fmt.Errorf("Error executing GET, response was %d", resp.StatusCode)
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return file.Name(), bosherr.WrapError(err, "Copying response to tempfile")
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return file.Name(), bosherr.WrapErrorf(err, "Rewinding file pointer to beginning")
	}

	err = digest.Verify(file)
	if err != nil {
		return file.Name(), bosherr.WrapErrorf(err, "Checking downloaded blob digest")
	}

	return file.Name(), nil
}

func isSuccess(resp *http.Response) bool {
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
