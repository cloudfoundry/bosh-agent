//go:build !windows
// +build !windows

package jobsupervisor

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshalert "github.com/cloudfoundry/bosh-agent/v2/agent/alert"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

const systemdJobSupervisorLogTag = "systemdJobSupervisor"
const boshJobUnitPrefix = "bosh-job-"
const boshJobsTarget = "bosh-jobs.target"
const systemdStopTimeout = 5 * time.Minute

type bpmProcess struct {
	Name       string            `yaml:"name"`
	Executable string            `yaml:"executable"`
	Args       []string          `yaml:"args"`
	Env        map[string]string `yaml:"env"`
	Limits     interface{}       `yaml:"limits"`
}

type bpmConfig struct {
	Processes []bpmProcess `yaml:"processes"`
}

type systemdJobSupervisor struct {
	fs          boshsys.FileSystem
	runner      boshsys.CmdRunner
	logger      boshlog.Logger
	dirProvider boshdir.Provider

	mu          sync.Mutex
	jobUnits    map[string][]string // jobName -> list of unit names
	failedUnits map[string]bool     // units already reported as failed (for edge-triggered alerts)
}

func NewSystemdJobSupervisor(
	fs boshsys.FileSystem,
	runner boshsys.CmdRunner,
	logger boshlog.Logger,
	dirProvider boshdir.Provider,
) JobSupervisor {
	return &systemdJobSupervisor{
		fs:          fs,
		runner:      runner,
		logger:      logger,
		dirProvider: dirProvider,
		jobUnits:    make(map[string][]string),
		failedUnits: make(map[string]bool),
	}
}

func (s *systemdJobSupervisor) Reload() error {
	_, _, _, err := s.runner.RunCommand("systemctl", "daemon-reload")
	if err != nil {
		return bosherr.WrapError(err, "Running systemctl daemon-reload")
	}
	return nil
}

func (s *systemdJobSupervisor) AddJob(jobName string, jobIndex int, configPath string) error {
	s.logger.Debug(systemdJobSupervisorLogTag, "Adding systemd job %s from %s", jobName, configPath)

	configContent, err := s.fs.ReadFile(configPath)
	if err != nil {
		return bosherr.WrapError(err, "Reading processes.yml")
	}

	var config bpmConfig
	if err := yaml.Unmarshal(configContent, &config); err != nil {
		return bosherr.WrapError(err, "Parsing processes.yml")
	}

	if len(config.Processes) == 0 {
		return bosherr.Error("processes.yml contains no processes")
	}

	// Ensure the grouping target exists before writing units that reference it.
	if err := s.ensureBoshJobsTarget(); err != nil {
		return err
	}

	var unitNames []string

	for i, process := range config.Processes {
		procName := process.Name
		if procName == "" {
			if i == 0 {
				procName = jobName
			} else {
				return bosherr.Errorf("Process at index %d has no name", i)
			}
		}

		unitName := s.unitName(jobName, procName)
		unitContent := s.generateUnit(jobName, procName, i == 0 && procName == jobName)
		unitPath := filepath.Join(s.dirProvider.SystemdDir(), unitName)

		if err := s.fs.WriteFileString(unitPath, unitContent); err != nil {
			return bosherr.WrapErrorf(err, "Writing systemd unit %s", unitName)
		}

		// Remove any stale unmonitor drop-in from a previous drain so that
		// Restart= is active again for the freshly configured unit.
		dropInPath := filepath.Join(s.dirProvider.SystemdDir(), unitName+".d", "unmonitor.conf")
		_ = s.fs.RemoveAll(dropInPath)

		unitNames = append(unitNames, unitName)
	}

	if _, _, _, err := s.runner.RunCommand("systemctl", "daemon-reload"); err != nil {
		return bosherr.WrapError(err, "Running systemctl daemon-reload")
	}

	for _, unitName := range unitNames {
		if _, _, _, err := s.runner.RunCommand("systemctl", "enable", unitName); err != nil {
			return bosherr.WrapErrorf(err, "Enabling systemd unit %s", unitName)
		}
	}

	s.mu.Lock()
	s.jobUnits[jobName] = unitNames
	s.mu.Unlock()

	return nil
}

// ensureBoshJobsTarget writes /etc/systemd/system/bosh-jobs.target if it does
// not already exist. The agent is the sole owner of this file; it is not
// provided by the stemcell.
func (s *systemdJobSupervisor) ensureBoshJobsTarget() error {
	targetPath := filepath.Join(s.dirProvider.SystemdDir(), boshJobsTarget)
	if s.fs.FileExists(targetPath) {
		return nil
	}
	s.logger.Info(systemdJobSupervisorLogTag, "bosh-jobs.target not found, writing it now")
	const targetContent = "[Unit]\nDescription=BOSH Jobs\nAllowIsolate=no\n"
	if err := s.fs.WriteFileString(targetPath, targetContent); err != nil {
		return bosherr.WrapError(err, "Writing bosh-jobs.target")
	}
	if _, _, _, err := s.runner.RunCommand("systemctl", "daemon-reload"); err != nil {
		return bosherr.WrapError(err, "Running daemon-reload after writing bosh-jobs.target")
	}
	return nil
}

func (s *systemdJobSupervisor) RemoveAllJobs() error {
	s.mu.Lock()
	units := s.allUnits()
	s.jobUnits = make(map[string][]string)
	s.mu.Unlock()

	for _, unitName := range units {
		_, _, _, _ = s.runner.RunCommand("systemctl", "disable", unitName)
		unitPath := filepath.Join(s.dirProvider.SystemdDir(), unitName)
		_ = s.fs.RemoveAll(unitPath)
		dropInDir := filepath.Join(s.dirProvider.SystemdDir(), unitName+".d")
		_ = s.fs.RemoveAll(dropInDir)
	}

	if len(units) > 0 {
		_, _, _, _ = s.runner.RunCommand("systemctl", "daemon-reload")
	}
	return nil
}

func (s *systemdJobSupervisor) Start() error {
	s.mu.Lock()
	hasJobs := len(s.jobUnits) > 0
	s.mu.Unlock()

	if !hasJobs {
		return nil
	}

	if _, _, _, err := s.runner.RunCommand("systemctl", "start", boshJobsTarget); err != nil {
		return bosherr.WrapError(err, "Starting bosh-jobs.target")
	}

	if err := s.removeStoppedFile(); err != nil {
		return bosherr.WrapError(err, "Removing stopped file")
	}

	return nil
}

func (s *systemdJobSupervisor) Stop() error {
	s.mu.Lock()
	units := s.allUnits()
	s.mu.Unlock()

	for _, unitName := range units {
		if _, _, _, err := s.runner.RunCommand("systemctl", "stop", unitName); err != nil {
			s.logger.Error(systemdJobSupervisorLogTag, "Failed to stop unit %s: %s", unitName, err.Error())
		}
	}

	if err := s.writeStoppedFile(); err != nil {
		return bosherr.WrapError(err, "Creating stopped file")
	}

	return nil
}

func (s *systemdJobSupervisor) StopAndWait() error {
	s.mu.Lock()
	units := s.allUnits()
	s.mu.Unlock()

	for _, unitName := range units {
		if _, _, _, err := s.runner.RunCommand("systemctl", "stop", unitName); err != nil {
			s.logger.Error(systemdJobSupervisorLogTag, "Failed to stop unit %s: %s", unitName, err.Error())
		}
	}

	deadline := time.Now().Add(systemdStopTimeout)
	for time.Now().Before(deadline) {
		allStopped := true
		for _, unitName := range units {
			stdout, _, _, err := s.runner.RunCommand("systemctl", "is-active", unitName)
			state := strings.TrimSpace(stdout)
			if err == nil && state == "active" {
				allStopped = false
				break
			}
			if state == "activating" || state == "deactivating" {
				allStopped = false
				break
			}
		}
		if allStopped {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err := s.writeStoppedFile(); err != nil {
		return bosherr.WrapError(err, "Creating stopped file")
	}

	return nil
}

func (s *systemdJobSupervisor) Unmonitor() error {
	s.mu.Lock()
	units := s.allUnits()
	s.mu.Unlock()

	// Disable automatic restart by writing a drop-in that overrides Restart=no.
	// This matches monit's "unmonitor" semantics: the service keeps running but
	// will not be restarted if it exits. The actual stop is done by Stop().
	for _, unitName := range units {
		dropInDir := filepath.Join(s.dirProvider.SystemdDir(), unitName+".d")
		if err := s.fs.MkdirAll(dropInDir, 0755); err != nil {
			return bosherr.WrapErrorf(err, "Creating drop-in dir for %s", unitName)
		}
		dropInPath := filepath.Join(dropInDir, "unmonitor.conf")
		if err := s.fs.WriteFileString(dropInPath, "[Service]\nRestart=no\n"); err != nil {
			return bosherr.WrapErrorf(err, "Writing unmonitor drop-in for %s", unitName)
		}
	}

	if len(units) > 0 {
		if _, _, _, err := s.runner.RunCommand("systemctl", "daemon-reload"); err != nil {
			return bosherr.WrapError(err, "Reloading systemd after unmonitor")
		}
	}

	return nil
}

func (s *systemdJobSupervisor) Status() string {
	s.mu.Lock()
	units := s.allUnits()
	hasJobs := len(s.jobUnits) > 0
	s.mu.Unlock()

	if !hasJobs {
		return "running"
	}

	if s.fs.FileExists(s.stoppedFilePath()) {
		return "stopped"
	}

	for _, unitName := range units {
		stdout, _, _, _ := s.runner.RunCommand("systemctl", "is-active", unitName)
		state := strings.TrimSpace(stdout)
		if state == "activating" {
			return "starting"
		}
	}

	for _, unitName := range units {
		stdout, _, _, _ := s.runner.RunCommand("systemctl", "is-active", unitName)
		state := strings.TrimSpace(stdout)
		if state != "active" {
			return "failing"
		}
	}

	return "running"
}

func (s *systemdJobSupervisor) Processes() ([]Process, error) {
	var processes []Process

	s.mu.Lock()
	units := s.allUnits()
	s.mu.Unlock()

	for _, unitName := range units {
		procName := s.processNameFromUnit(unitName)

		stdout, _, _, _ := s.runner.RunCommand("systemctl", "is-active", unitName)
		state := strings.TrimSpace(stdout)
		status := s.mapSystemdState(state)

		var uptime int
		var memKb int
		var cpuTotal float64

		propStdout, _, _, err := s.runner.RunCommand("systemctl", "show",
			"--property=MainPID,MemoryCurrent,ActiveEnterTimestampMonotonic", unitName)
		if err == nil {
			uptime, memKb, cpuTotal = s.parseUnitProperties(propStdout)
		}

		processes = append(processes, Process{
			Name:   procName,
			State:  status,
			Uptime: UptimeVitals{Secs: uptime},
			Memory: MemoryVitals{Kb: memKb},
			CPU:    CPUVitals{Total: cpuTotal},
		})
	}

	return processes, nil
}

func (s *systemdJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	s.logger.Debug(systemdJobSupervisorLogTag, "MonitorJobFailures: starting failure poll (edge-triggered)")

	go s.pollForFailures(handler)

	return nil
}

// pollForFailures polls unit states and fires the handler only on
// transitions into the failed state (edge-triggered), matching monit's
// behavior of sending a single SMTP alert per failure event.
func (s *systemdJobSupervisor) pollForFailures(handler JobFailureHandler) {
	for {
		time.Sleep(5 * time.Second)

		s.mu.Lock()
		units := s.allUnits()
		s.mu.Unlock()

		for _, unitName := range units {
			stdout, _, _, _ := s.runner.RunCommand("systemctl", "is-failed", unitName)
			isFailed := strings.TrimSpace(stdout) == "failed"

			s.mu.Lock()
			alreadyReported := s.failedUnits[unitName]

			if isFailed && !alreadyReported {
				s.failedUnits[unitName] = true
				s.mu.Unlock()

				procName := s.processNameFromUnit(unitName)
				alert := boshalert.JobFailureAlert{
					ID:          unitName,
					Service:     procName,
					Event:       "pid failed",
					Action:      "restart",
					Date:        time.Now().Format(time.RFC1123Z),
					Description: fmt.Sprintf("systemd unit %s entered failed state", unitName),
				}

				if err := handler(alert); err != nil {
					s.logger.Error(systemdJobSupervisorLogTag, "Failure handler error for %s: %s", unitName, err.Error())
				}
			} else if !isFailed && alreadyReported {
				delete(s.failedUnits, unitName)
				s.mu.Unlock()
			} else {
				s.mu.Unlock()
			}
		}
	}
}

func (s *systemdJobSupervisor) HealthRecorder(status string) {
}

func (s *systemdJobSupervisor) unitName(jobName, processName string) string {
	if processName == jobName {
		return fmt.Sprintf("%s%s.service", boshJobUnitPrefix, jobName)
	}
	return fmt.Sprintf("%s%s-%s.service", boshJobUnitPrefix, jobName, processName)
}

func (s *systemdJobSupervisor) processNameFromUnit(unitName string) string {
	name := strings.TrimPrefix(unitName, boshJobUnitPrefix)
	name = strings.TrimSuffix(name, ".service")
	return name
}

func (s *systemdJobSupervisor) generateUnit(jobName, processName string, isDefault bool) string {
	bpmCmd := "/var/vcap/jobs/bpm/bin/bpm"

	var execStartPre, execStart, execStop string
	if isDefault {
		execStartPre = fmt.Sprintf("ExecStartPre=-/%s stop %s", bpmCmd, jobName)
		execStart = fmt.Sprintf("ExecStart=/%s run %s", bpmCmd, jobName)
		execStop = fmt.Sprintf("ExecStop=/%s stop %s", bpmCmd, jobName)
	} else {
		execStartPre = fmt.Sprintf("ExecStartPre=-/%s stop %s -p %s", bpmCmd, jobName, processName)
		execStart = fmt.Sprintf("ExecStart=/%s run %s -p %s", bpmCmd, jobName, processName)
		execStop = fmt.Sprintf("ExecStop=/%s stop %s -p %s", bpmCmd, jobName, processName)
	}

	return fmt.Sprintf(`[Unit]
Description=BOSH job %s process %s
PartOf=%s
After=network.target

[Service]
Type=simple
%s
%s
%s
Restart=always
RestartSec=5
TimeoutStartSec=30
TimeoutStopSec=30

[Install]
WantedBy=%s
`, jobName, processName, boshJobsTarget, execStartPre, execStart, execStop, boshJobsTarget)
}

func (s *systemdJobSupervisor) allUnits() []string {
	var units []string
	for _, jobUnits := range s.jobUnits {
		units = append(units, jobUnits...)
	}
	return units
}

func (s *systemdJobSupervisor) HasJobs() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobUnits) > 0
}

func (s *systemdJobSupervisor) stoppedFilePath() string {
	return filepath.Join(s.dirProvider.MonitDir(), "systemd_stopped")
}

func (s *systemdJobSupervisor) writeStoppedFile() error {
	return s.fs.WriteFileString(s.stoppedFilePath(), "")
}

func (s *systemdJobSupervisor) removeStoppedFile() error {
	return s.fs.RemoveAll(s.stoppedFilePath())
}

func (s *systemdJobSupervisor) mapSystemdState(state string) string {
	switch state {
	case "active":
		return "running"
	case "activating":
		return "starting"
	case "inactive", "deactivating":
		return "stopped"
	case "failed":
		return "failing"
	default:
		return "unknown"
	}
}

func (s *systemdJobSupervisor) parseUnitProperties(propOutput string) (uptime int, memKb int, cpuTotal float64) {
	for _, line := range strings.Split(propOutput, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		switch key {
		case "MemoryCurrent":
			if value != "[not set]" && value != "" {
				var bytes uint64
				if _, err := fmt.Sscanf(value, "%d", &bytes); err == nil {
					memKb = int(bytes / 1024)
				}
			}
		case "ActiveEnterTimestampMonotonic":
			if value != "" && value != "0" {
				var usec uint64
				if _, err := fmt.Sscanf(value, "%d", &usec); err == nil {
					uptimeUsec := uint64(time.Now().UnixMicro()) - usec
					uptime = int(uptimeUsec / 1_000_000)
					if uptime < 0 {
						uptime = 0
					}
				}
			}
		}
	}
	_ = cpuTotal
	return
}
