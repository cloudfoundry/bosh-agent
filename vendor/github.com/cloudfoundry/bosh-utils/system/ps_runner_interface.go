package system

type PSCommand struct {
	Script string
}

type PSRunner interface {
	RunCommand(PSCommand) (string, string, error)
}
