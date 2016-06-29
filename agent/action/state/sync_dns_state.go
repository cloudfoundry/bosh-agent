package state

import (
	"encoding/json"
	//"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type SyncDNSState interface {
	LoadState() (BlobstoreDNSState, error)
	SaveState(blobstoreDNSState BlobstoreDNSState) error
}

type BlobstoreDNSState struct {
	Version uint32 `json:"version"`
}

type syncDNSState struct {
	fs   boshsys.FileSystem
	path string
}

func NewSyncDNSState(fs boshsys.FileSystem, path string) *syncDNSState {
	return &syncDNSState{
		fs:   fs,
		path: path,
	}
}

func (s *syncDNSState) LoadState() (BlobstoreDNSState, error) {
	contents, err := s.fs.ReadFile(s.path)
	if err != nil {
		return BlobstoreDNSState{}, bosherr.WrapError(err, "reading state file")
	}

	bDNSState := BlobstoreDNSState{}
	err = json.Unmarshal(contents, &bDNSState)
	if err != nil {
		return BlobstoreDNSState{}, bosherr.WrapError(err, "unmarshalling state file")
	}

	return bDNSState, nil
}

func (s *syncDNSState) SaveState(blobstoreDNSState BlobstoreDNSState) error {
	contents, err := json.Marshal(blobstoreDNSState)
	if err != nil {
		return bosherr.WrapError(err, "marshalling blobstore DNS state")
	}

	err = s.fs.WriteFile(s.path, contents)
	if err != nil {
		return bosherr.WrapError(err, "writing the blobstore DNS state")
	}

	return nil
}
