package drain

type Script interface {
	Exists() bool
	Run(params ScriptParams) (value int, err error)
	Path() string
}
