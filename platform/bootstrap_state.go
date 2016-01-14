package platform

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type BootstrapState struct {
	Linux LinuxState
	path  string
	fs    boshsys.FileSystem
}

type LinuxState struct {
	HostsConfigured    bool `json:"hosts_configured"`
	HostnameConfigured bool `json:"hostname_configured"`
}

func NewBootstrapState(fs boshsys.FileSystem, path string) (*BootstrapState, error) {
	state := BootstrapState{fs: fs, path: path}

	if !fs.FileExists(path) {
		return &state, nil
	}

	bytes, err := fs.ReadFile(path)
	if err != nil {
		return nil, bosherr.WrapError(err, "Reading state file")
	}

	err = json.Unmarshal(bytes, &state)
	if err != nil {
		return nil, bosherr.WrapError(err, "Unmarshalling state file")
	}

	return &state, nil
}

func (s *BootstrapState) SaveState() (err error) {
	jsonState, err := json.Marshal(*s)
	if err != nil {
		return bosherr.WrapError(err, "Marshalling state file")
	}

	err = s.fs.WriteFile(s.path, jsonState)
	if err != nil {
		return bosherr.WrapError(err, "Writing state file")
	}

	return
}
