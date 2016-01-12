package app

import (
	"encoding/json"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type State struct {
	HostsConfigured    bool `json:"hosts_configured"`
	HostnameConfigured bool `json:"hostname_configured"`
}

func SaveState(fs boshsys.FileSystem, path string, newState State) error {
	jsonState, _ := json.Marshal(newState)

	err := fs.WriteFile(path, jsonState)
	if err != nil {
		return bosherr.WrapError(err, "Writing file")
	}

	return nil
}

func LoadState(fs boshsys.FileSystem, path string) (State, error) {
	var state State

	bytes, err := fs.ReadFile(path)
	if err != nil {
		return state, bosherr.WrapError(err, "Reading file")
	}

	err = json.Unmarshal(bytes, &state)
	if err != nil {
		return state, bosherr.WrapError(err, "Loading file")
	}

	return state, nil
}
