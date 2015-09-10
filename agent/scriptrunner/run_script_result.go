package scriptrunner

type RunScriptResult struct {
	JobName    string
	ScriptPath string
	Error      error
}
