package logstarprovider

//go:generate counterfeiter . LogsTarProvider

type LogsTarProvider interface {
	Get(logType string, filters []string) (string, error)
	CleanUp(path string) error
}
