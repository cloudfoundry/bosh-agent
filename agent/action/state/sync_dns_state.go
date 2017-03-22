package state

import (
	"encoding/json"

	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
)

type SyncDNSState interface {
	StateFileExists() bool
	LoadState() (LocalDNSState, error)
	SaveState(localDNSState LocalDNSState) error
}

type LocalDNSState struct {
	Version     uint64      `json:"version"`
	Records     [][2]string `json:"records"`
	RecordKeys  []string    `json:"record_keys"`
	RecordInfos [][]string  `json:"record_infos"`
}

type syncDNSState struct {
	fs            boshsys.FileSystem
	path          string
	uuidGenerator boshuuid.Generator
}

func NewSyncDNSState(platform boshplatform.Platform, path string, generator boshuuid.Generator) SyncDNSState {
	return &syncDNSState{
		fs:            platform.GetFs(),
		path:          path,
		uuidGenerator: generator,
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

	uuid, err := s.uuidGenerator.Generate()
	if err != nil {
		return bosherr.WrapError(err, "generating uuid for temp file")
	}

	tmpFilePath := s.path + uuid

	if err := s.fs.WriteFile(tmpFilePath, contents); err != nil {
		return bosherr.WrapError(err, "writing the blobstore DNS state")
	}

	if err := s.fs.Rename(tmpFilePath, s.path); err != nil {
		return bosherr.WrapError(err, "renaming")
	}

	return nil
}

func (s *syncDNSState) StateFileExists() bool {
	return s.fs.FileExists(s.path)
}
