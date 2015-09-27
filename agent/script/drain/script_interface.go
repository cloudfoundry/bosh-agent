package drain

//go:generate counterfeiter . ScriptProvider

type ScriptProvider interface {
	NewScript(jobName string, params ScriptParams) Script
}

type Script interface {
	Tag() string
	Path() string

	Exists() bool
	Run() error
}
