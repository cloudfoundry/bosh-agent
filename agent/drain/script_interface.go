package drain

//go:generate counterfeiter . ScriptProvider

type ScriptProvider interface {
	NewScript(templateName string) Script
}

type Script interface {
	Exists() bool
	Run(ScriptParams) error
	Path() string
}
