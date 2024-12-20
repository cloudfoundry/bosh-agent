package cmd

import (
	boshenv "github.com/cloudfoundry/bosh-agent/v2/agent/script/pathenv"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func BuildCommand(path string) boshsys.Command {
	return boshsys.Command{
		Name: "powershell",
		Args: []string{path},
		Env: map[string]string{
			"PATH": boshenv.Path(),
		},
	}
}
