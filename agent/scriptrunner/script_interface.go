package scriptrunner

type Script interface {
	Exists() bool
	Run() (value int, err error)
}
