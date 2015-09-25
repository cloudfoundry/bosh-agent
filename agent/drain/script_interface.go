package drain

type Script interface {
	Exists() bool
	Run(params ScriptParams) error
	Path() string
}
