package action

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-utils/blobstore"
)

type UploadBlobSpec struct {
	BlobID  string `json:"blob_id"`
	Sha1    string `json:"sha1"`
	Payload string `json:"payload"`
}

type UploadBlobAction struct {
	blobManager blobstore.BlobManagerInterface
}

func NewUploadBlobAction(blobManager blobstore.BlobManagerInterface) UploadBlobAction {
	return UploadBlobAction{blobManager: blobManager}
}

func (a UploadBlobAction) IsAsynchronous() bool {
	return true
}

func (a UploadBlobAction) IsPersistent() bool {
	return false
}

func (a UploadBlobAction) IsLoggable() bool {
	return false
}

func (a UploadBlobAction) Run(content UploadBlobSpec) (string, error) {

	decodedPayload, err := base64.StdEncoding.DecodeString(content.Payload)
	if err != nil {
		return content.BlobID, err
	}

	if err = a.validatePayload(decodedPayload, content.Sha1); err != nil {
		return content.BlobID, err
	}

	reader := bytes.NewReader(decodedPayload)

	err = a.blobManager.Write(content.BlobID, reader)

	return content.BlobID, err
}

func (a UploadBlobAction) validatePayload(payload []byte, payloadSha1 string) error {

	h := sha1.New()
	h.Write(payload)
	computedShaHex := h.Sum(nil)

	computedHash := hex.EncodeToString(computedShaHex)

	if computedHash != payloadSha1 {
		return fmt.Errorf("Payload corrupted. SHA1 mismatch. Expected %s but received %s", payloadSha1, computedHash)
	}

	return nil
}

func (a UploadBlobAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a UploadBlobAction) Cancel() error {
	return errors.New("not supported")
}
