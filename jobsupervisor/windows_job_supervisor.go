package jobsupervisor

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const (
	serviceDescription = "vcap"

	serviceWrapperExeFileName    = "job-service-wrapper.exe"
	serviceWrapperConfigFileName = "job-service-wrapper.xml"

	startJobScript = `
(get-wmiobject win32_service -filter "description='` + serviceDescription + `'") | ForEach{ Start-Service $_.Name }
`
	stopJobScript = `
(get-wmiobject win32_service -filter "description='` + serviceDescription + `'") | ForEach{ Stop-Service $_.Name }
`
	deleteAllJobsScript = `
(get-wmiobject win32_service -filter "description='` + serviceDescription + `'") | ForEach{ $_.delete() }
`
	getStatusScript = `
(get-wmiobject win32_service -filter "description='` + serviceDescription + `'") | ForEach{ $_.State }
`
	serviceWrapperTemplate = `
<service>
  <id>{{ .ID }}</id>
  <name>{{ .Name }}</name>
  <description>` + serviceDescription + `</description>
  <executable>{{ .Executable }}</executable>
  <arguments>{{ .Arguments }}</arguments>
  <logpath>{{ .LogPath }}</logpath>
  <log mode="append"/>
  <onfailure action="restart" delay="5 sec"/>
</service>
`
)

type WindowsServiceWrapperConfig struct {
	ID         string
	Name       string
	Executable string
	Arguments  string
	LogPath    string
}

type WindowsProcessConfig struct {
	Processes []WindowsProcess `json:"processes"`
}

type WindowsProcess struct {
	Name       string   `json:"name"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

type windowsJobSupervisor struct {
	cmdRunner   boshsys.CmdRunner
	dirProvider boshdirs.Provider
	fs          boshsys.FileSystem
	logger      boshlog.Logger
	logTag      string
}

func NewWindowsJobSupervisor(
	cmdRunner boshsys.CmdRunner,
	dirProvider boshdirs.Provider,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) JobSupervisor {
	return &windowsJobSupervisor{
		cmdRunner:   cmdRunner,
		dirProvider: dirProvider,
		fs:          fs,
		logger:      logger,
		logTag:      "windowsJobSupervisor",
	}
}

func (s *windowsJobSupervisor) Reload() error {
	return nil
}

func (s *windowsJobSupervisor) Start() error {
	_, _, _, err := s.cmdRunner.RunCommand("powershell", "-noprofile", "-noninteractive", "/C", startJobScript)
	if err != nil {
		return bosherr.WrapError(err, "Starting windows job process")
	}

	err = s.fs.RemoveAll(s.stoppedFilePath())
	if err != nil {
		return bosherr.WrapError(err, "Removing stopped file")
	}

	return nil
}

func (s *windowsJobSupervisor) Stop() error {
	_, _, _, err := s.cmdRunner.RunCommand("powershell", "-noprofile", "-noninteractive", "/C", stopJobScript)
	if err != nil {
		return bosherr.WrapError(err, "Stopping services")
	}

	err = s.fs.WriteFileString(s.stoppedFilePath(), "")
	if err != nil {
		return bosherr.WrapError(err, "Creating stopped file")
	}

	return nil
}

func (s *windowsJobSupervisor) Unmonitor() error {
	return nil
}

func (s *windowsJobSupervisor) Status() (status string) {
	if s.fs.FileExists(s.stoppedFilePath()) {
		return "stopped"
	}

	stdout, _, _, err := s.cmdRunner.RunCommand("powershell", "-noprofile", "-noninteractive", "/C", getStatusScript)
	if err != nil {
		return "failing"
	}

	stdout = strings.TrimSpace(stdout)
	if len(stdout) == 0 {
		s.logger.Debug(s.logTag, "No statuses reported for job processes")
		return "running"
	}

	statuses := strings.Split(stdout, "\r\n")
	s.logger.Debug(s.logTag, "Got statuses %#v", statuses)

	for _, status := range statuses {
		if status != "Running" {
			return "failing"
		}
	}

	return "running"
}

func (s *windowsJobSupervisor) Processes() ([]Process, error) {
	return []Process{}, nil
}

func (s *windowsJobSupervisor) AddJob(jobName string, jobIndex int, configPath string) error {
	configFileContents, err := s.fs.ReadFile(configPath)
	if err != nil {
		return err
	}

	if len(configFileContents) == 0 {
		s.logger.Debug(s.logTag, "Skipping job configuration for %q, empty monit config file %q", jobName, configPath)
		return nil
	}

	var processConfig WindowsProcessConfig
	err = json.Unmarshal(configFileContents, &processConfig)
	if err != nil {
		return err
	}

	for _, process := range processConfig.Processes {
		logPath := path.Join(s.dirProvider.LogsDir(), jobName, process.Name)
		err := s.fs.MkdirAll(logPath, os.FileMode(0750))
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating log directory for service '%s'", process.Name)
		}

		serviceConfig := WindowsServiceWrapperConfig{
			ID:         process.Name,
			Name:       process.Name,
			Executable: process.Executable,
			Arguments:  strings.Join(process.Args, " "),
			LogPath:    logPath,
		}

		buffer := bytes.NewBuffer([]byte{})
		t := template.Must(template.New("service-wrapper-config").Parse(serviceWrapperTemplate))
		err = t.Execute(buffer, serviceConfig)
		if err != nil {
			return bosherr.WrapErrorf(err, "Rendering service config template for service '%s'", process.Name)
		}

		s.logger.Debug(s.logTag, "Configuring service wrapper for job %q with configPath %q", jobName, configPath)

		jobDir := filepath.Dir(configPath)

		processDir := filepath.Join(jobDir, process.Name)
		err = s.fs.MkdirAll(processDir, os.FileMode(0750))
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating job directory for service '%s' at '%s'", process.Name, processDir)
		}

		serviceWrapperConfigFile := filepath.Join(processDir, serviceWrapperConfigFileName)
		err = s.fs.WriteFile(serviceWrapperConfigFile, buffer.Bytes())
		if err != nil {
			return bosherr.WrapErrorf(err, "Saving service config file for service '%s'", process.Name)
		}

		serviceWrapperExePath := filepath.Join(s.dirProvider.BoshBinDir(), serviceWrapperExeFileName)
		err = s.fs.CopyFile(serviceWrapperExePath, filepath.Join(processDir, serviceWrapperExeFileName))
		if err != nil {
			return bosherr.WrapErrorf(err, "Copying service wrapper in job directory '%s'", processDir)
		}

		cmdToRun := filepath.Join(processDir, serviceWrapperExeFileName)

		_, _, _, err = s.cmdRunner.RunCommand(cmdToRun, "install")
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating service '%s'", process.Name)
		}
	}

	return nil
}

func (s *windowsJobSupervisor) RemoveAllJobs() error {
	_, _, _, err := s.cmdRunner.RunCommand("powershell", "-noprofile", "-noninteractive", "/C", deleteAllJobsScript)
	return err
}

func (s *windowsJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	return nil
}

func (s *windowsJobSupervisor) stoppedFilePath() string {
	return filepath.Join(s.dirProvider.MonitDir(), "stopped")
}
