//go:build !windows
// +build !windows

package cmd

import (
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshenv "github.com/cloudfoundry/bosh-agent/v2/agent/script/pathenv"
)

func BuildCommand(path string) boshsys.Command {
	return boshsys.Command{
		Name: path,
		Env: map[string]string{
			"PATH": boshenv.Path(),
		},
	}
}
