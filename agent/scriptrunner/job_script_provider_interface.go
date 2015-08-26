package scriptrunner

//go:generate counterfeiter . JobScriptProvider

type JobScriptProvider interface {
	Get(jobName string, relativePath string) Script
}
