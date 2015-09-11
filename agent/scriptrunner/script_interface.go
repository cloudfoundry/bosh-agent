package scriptrunner

//go:generate counterfeiter . Script

type Script interface {
	Exists() bool
	Run(resultChannel chan RunScriptResult)
	Path() string
	LogPath() string
	JobName() string
}
