package drain

//go:generate counterfeiter . ScriptProvider

type ScriptProvider interface {
	NewScript(templateName string) Script
}

type Script interface {
	Path() string

	Exists() bool
	Run(ScriptParams) error
}
