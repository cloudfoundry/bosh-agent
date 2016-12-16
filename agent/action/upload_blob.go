package action

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-utils/blobstore"
	"github.com/cloudfoundry/bosh-utils/crypto"
)

type UploadBlobSpec struct {
	BlobID   string                    `json:"blob_id"`
	Checksum crypto.MultipleDigestImpl `json:"checksum"`
	Payload  string                    `json:"payload"`
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

	if err = a.validatePayload(decodedPayload, content.Checksum); err != nil {
		return content.BlobID, err
	}

	reader := bytes.NewReader(decodedPayload)

	err = a.blobManager.Write(content.BlobID, reader)

	return content.BlobID, err
}

func (a UploadBlobAction) validatePayload(payload []byte, payloadDigest crypto.Digest) error {
	actualDigest, err := payloadDigest.Algorithm().CreateDigest(bytes.NewReader(payload))
	if err != nil {
		return err
	}

	//TODO: payloadDigest.Verify(bytes.NewReader(payload))
	err = payloadDigest.Verify(actualDigest)
	if err != nil {
		return fmt.Errorf("Payload corrupted. Checksum mismatch. Expected '%s' but received '%s'", payloadDigest.String(), actualDigest.String())
	}

	return nil
}

func (a UploadBlobAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a UploadBlobAction) Cancel() error {
	return errors.New("not supported")
}
