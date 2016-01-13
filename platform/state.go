package platform

import (
	"encoding/json"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type State struct {
	Linux LinuxState
	path  string
	fs    boshsys.FileSystem
}

type LinuxState struct {
	HostsConfigured    bool `json:"hosts_configured"`
	HostnameConfigured bool `json:"hostname_configured"`
}

func NewState(fs boshsys.FileSystem, path string) (*State, error) {
	state := State{fs: fs, path: path}

	if !fs.FileExists(path) {
		return &state, nil
	}

	bytes, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &state)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *State) SaveState() (err error) {
	jsonState, err := json.Marshal(*s)
	if err != nil {
		return
	}

	err = s.fs.WriteFile(s.path, jsonState)
	if err != nil {
		return
	}

	return
}
