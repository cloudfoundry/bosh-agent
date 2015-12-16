package fakes

import boshsys "github.com/cloudfoundry/bosh-utils/system"

type FakePSRunner struct {
	RunCommands   []boshsys.PSCommand
	RunCommandErr error
}

func NewFakePSRunner() *FakePSRunner {
	return &FakePSRunner{
		RunCommands: []boshsys.PSCommand{},
	}
}

func (r *FakePSRunner) RunCommand(cmd boshsys.PSCommand) (string, string, error) {
	r.RunCommands = append(r.RunCommands, cmd)
	return "", "", r.RunCommandErr
}
