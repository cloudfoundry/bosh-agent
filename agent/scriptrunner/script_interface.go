package scriptrunner

//go:generate counterfeiter . Script

type Script interface {
	Exists() bool
	Run(errorChan chan RunScriptResult, doneChan chan RunScriptResult)
	Path() string
	LogPath() string
	JobName() string
}
