package app

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshinf "github.com/cloudfoundry/bosh-agent/infrastructure"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type Config struct {
	Platform       boshplatform.Options
	Infrastructure boshinf.Options
}

func LoadConfigFromPath(fs boshsys.FileSystem, path string) (Config, error) {
	var config Config

	if path == "" {
		return config, nil
	}

	bytes, err := fs.ReadFile(path)
	if err != nil {
		return config, bosherr.WrapError(err, "Reading file")
	}

	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return config, bosherr.WrapError(err, "Loading file")
	}

	return config, nil
}
