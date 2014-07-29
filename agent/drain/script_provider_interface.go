package drain

type ScriptProvider interface {
	NewScript(templateName string) (drainScript Script)
}
