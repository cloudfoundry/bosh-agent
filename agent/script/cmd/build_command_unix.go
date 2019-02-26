// +build !windows

package cmd

import (
	boshenv "github.com/cloudfoundry/bosh-agent/agent/script/pathenv"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

func BuildCommand(path string, env map[string]string) boshsys.Command {
	if env == nil {
		env = map[string]string{}
	}

	env["PATH"] = boshenv.Path()

	return boshsys.Command{
		Name: path,
		Env:  env,
	}
}
