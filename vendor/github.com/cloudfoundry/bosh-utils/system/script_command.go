package system

type ScriptCommandFactory interface {
	New(path string, args ...string) Command
}

func NewScriptCommandFactory(platformName string) ScriptCommandFactory {
	if platformName == "windows" {
		return &psScriptCommandFactory{}
	}

	return &linuxScriptCommandFactory{}
}

type linuxScriptCommandFactory struct{}

func (s *linuxScriptCommandFactory) New(path string, args ...string) Command {
	return Command{
		Name: path,
		Args: args,
	}
}

type psScriptCommandFactory struct{}

func (s *psScriptCommandFactory) New(path string, args ...string) Command {
	return Command{
		Name: "powershell",
		Args: append([]string{"-noprofile", "-noninteractive", path}, args...),
	}
}
