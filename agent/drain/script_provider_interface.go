package drain

//go:generate counterfeiter . ScriptProvider

type ScriptProvider interface {
	NewScript(templateName string) (drainScript Script)
}
