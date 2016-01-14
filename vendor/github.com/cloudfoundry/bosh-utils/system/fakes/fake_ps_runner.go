package fakes

import (
	"fmt"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type FakePSRunner struct {
	RunCommands      []boshsys.PSCommand
	RunCommandErr    error
	runCommandErrors map[string]error
}

func NewFakePSRunner() *FakePSRunner {
	return &FakePSRunner{
		RunCommands:      []boshsys.PSCommand{},
		runCommandErrors: map[string]error{},
	}
}

func (r *FakePSRunner) RunCommand(cmd boshsys.PSCommand) (string, string, error) {
	r.RunCommands = append(r.RunCommands, cmd)
	if err := r.runCommandErrors[cmd.Script]; err != nil {
		return "", "", err
	}
	return "", "", r.RunCommandErr
}

func (r *FakePSRunner) RegisterRunCommandError(script string, err error) {
	if _, ok := r.runCommandErrors[script]; ok {
		panic(fmt.Sprintf("RunCommand error is already set for command: %s", script))
	}
	r.runCommandErrors[script] = err
}
