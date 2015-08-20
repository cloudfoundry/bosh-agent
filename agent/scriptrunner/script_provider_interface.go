package scriptrunner

type ScriptProvider interface {
	Get(scriptPath string) (script Script)
}
