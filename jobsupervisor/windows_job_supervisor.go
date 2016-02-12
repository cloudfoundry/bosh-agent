package jobsupervisor

import (
	"encoding/json"
	"fmt"
	"strings"

	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const (
	addJobScript = `
New-Service -Name "%s" -Description "%s" -binaryPathName "%s"
`

	deleteAllJobsScript = `
(get-wmiobject win32_service -filter "description='%s'").delete()
`
)

type WindowsProcessConfig struct {
	Processes []WindowsProcess `json:"processes"`
}

type WindowsProcess struct {
	Name       string   `json:"name"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

type windowsJobSupervisor struct {
	processes []Process
	status    string
	cmdRunner boshsys.CmdRunner
	fs        boshsys.FileSystem
}

func NewWindowsJobSupervisor(cmdRunner boshsys.CmdRunner, fs boshsys.FileSystem) JobSupervisor {
	return &windowsJobSupervisor{
		status:    "unmonitored",
		cmdRunner: cmdRunner,
		fs:        fs,
	}
}

func (s *windowsJobSupervisor) Reload() error {
	return nil
}

func (s *windowsJobSupervisor) Start() error {
	s.processes = []Process{}
	s.status = "running"
	return nil
}

func (s *windowsJobSupervisor) Stop() error {
	s.status = "stopped"
	return nil
}

func (s *windowsJobSupervisor) Unmonitor() error {
	s.status = "unmonitored"
	return nil
}

func (s *windowsJobSupervisor) Status() (status string) {
	return s.status
}

func (s *windowsJobSupervisor) Processes() ([]Process, error) {
	return s.processes, nil
}

func (s *windowsJobSupervisor) AddJob(jobName string, jobIndex int, configPath string) error {
	configFileContents, err := s.fs.ReadFile(configPath)
	if err != nil {
		return err
	}

	var processConfig WindowsProcessConfig
	err = json.Unmarshal(configFileContents, &processConfig)
	if err != nil {
		return err
	}

	for _, process := range processConfig.Processes {
		args := strings.Join(process.Args, " ")
		commandToRun := process.Executable + " " + args
		psScript := fmt.Sprintf(addJobScript, process.Name, "vcap", commandToRun)

		_, _, _, err := s.cmdRunner.RunCommand("powershell", "-noprofile", "-noninteractive", "/C", psScript)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *windowsJobSupervisor) RemoveAllJobs() error {
	psScript := fmt.Sprintf(deleteAllJobsScript, "vcap")
	_, _, _, err := s.cmdRunner.RunCommand("powershell", "-noprofile", "-noninteractive", "/C", psScript)
	return err
}

func (s *windowsJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	return nil
}
