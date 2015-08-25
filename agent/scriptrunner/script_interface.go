package scriptrunner

//go:generate counterfeiter . Script

type Script interface {
	Exists() bool
	Run() (stdout string, stderr string, err error)
	Path() string
}
