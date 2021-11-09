package script

import (
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/script/drain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . JobScriptProvider

type JobScriptProvider interface {
	NewScript(jobName string, scriptName string, scriptEnv map[string]string) Script
	NewDrainScript(jobName string, params boshdrain.ScriptParams) CancellableScript
	NewParallelScript(scriptName string, scripts []Script) CancellableScript
}

//counterfeiter:generate . Script

type Script interface {
	Tag() string
	Path() string

	Exists() bool
	Run() error
}

//counterfeiter:generate . CancellableScript

type CancellableScript interface {
	Script
	Cancel() error
}
