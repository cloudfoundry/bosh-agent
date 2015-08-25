package scriptrunner

//go:generate counterfeiter . ScriptProvider

type ScriptProvider interface {
	Get(scriptPath string) (script Script)
}
