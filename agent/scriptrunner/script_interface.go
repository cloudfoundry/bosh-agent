package scriptrunner

//go:generate counterfeiter . JobScriptProvider

type JobScriptProvider interface {
	Get(jobName string, relativePath string) Script
}

//go:generate counterfeiter . Script

type Script interface {
	Exists() bool
	Run() ScriptResult
}

type ScriptResult struct {
	Tag        string
	ScriptPath string

	Error error
}
