package scriptrunner

//go:generate counterfeiter . Script

type Script interface {
	Tag() string
	Path() string
	Exists() bool
	Run(resultChannel chan RunScriptResult)
}
