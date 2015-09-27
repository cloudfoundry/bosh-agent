package script

//go:generate counterfeiter . JobScriptProvider

type JobScriptProvider interface {
	Get(jobName string, relativePath string) Script
}

//go:generate counterfeiter . Script

type Script interface {
	Tag() string
	Path() string

	Exists() bool
	Run() error
}
