package state

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type SyncDNSState interface {
	StateFileExists() bool
	LoadState() (LocalDNSState, error)
	SaveState(localDNSState LocalDNSState) error
}

type LocalDNSState struct {
	Version uint64 `json:"version"`
}

type syncDNSState struct {
	fs   boshsys.FileSystem
	path string
}

func NewSyncDNSState(fs boshsys.FileSystem, path string) SyncDNSState {
	return &syncDNSState{
		fs:   fs,
		path: path,
	}
}

func (s *syncDNSState) LoadState() (LocalDNSState, error) {
	contents, err := s.fs.ReadFile(s.path)
	if err != nil {
		return LocalDNSState{}, bosherr.WrapError(err, "reading state file")
	}

	bDNSState := LocalDNSState{}
	err = json.Unmarshal(contents, &bDNSState)
	if err != nil {
		return LocalDNSState{}, bosherr.WrapError(err, "unmarshalling state file")
	}

	return bDNSState, nil
}

func (s *syncDNSState) SaveState(localDNSState LocalDNSState) error {
	contents, err := json.Marshal(localDNSState)
	if err != nil {
		return bosherr.WrapError(err, "marshalling blobstore DNS state")
	}

	err = s.fs.WriteFile(s.path, contents)
	if err != nil {
		return bosherr.WrapError(err, "writing the blobstore DNS state")
	}

	return nil
}

func (s *syncDNSState) StateFileExists() bool {
	return s.fs.FileExists(s.path)
}
