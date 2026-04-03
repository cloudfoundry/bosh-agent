//go:build !windows
// +build !windows

package jobsupervisor

import (
	"strings"
	"sync"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const compositeJobSupervisorLogTag = "compositeJobSupervisor"

type compositeJobSupervisor struct {
	monitSupervisor   JobSupervisor
	systemdSupervisor JobSupervisor
	logger            boshlog.Logger

	mu              sync.Mutex
	monitJobCount   int
	systemdJobCount int
}

func NewCompositeJobSupervisor(
	monitSupervisor JobSupervisor,
	systemdSupervisor JobSupervisor,
	logger boshlog.Logger,
) JobSupervisor {
	return &compositeJobSupervisor{
		monitSupervisor:  monitSupervisor,
		systemdSupervisor: systemdSupervisor,
		logger:           logger,
	}
}

func (c *compositeJobSupervisor) Reload() error {
	if err := c.monitSupervisor.Reload(); err != nil {
		return err
	}
	return c.systemdSupervisor.Reload()
}

func (c *compositeJobSupervisor) AddJob(jobName string, jobIndex int, configPath string) error {
	if strings.HasSuffix(configPath, "processes.yml") {
		c.mu.Lock()
		c.systemdJobCount++
		c.mu.Unlock()
		return c.systemdSupervisor.AddJob(jobName, jobIndex, configPath)
	}

	c.mu.Lock()
	c.monitJobCount++
	c.mu.Unlock()
	return c.monitSupervisor.AddJob(jobName, jobIndex, configPath)
}

func (c *compositeJobSupervisor) RemoveAllJobs() error {
	if err := c.monitSupervisor.RemoveAllJobs(); err != nil {
		return err
	}
	if err := c.systemdSupervisor.RemoveAllJobs(); err != nil {
		return err
	}

	c.mu.Lock()
	c.monitJobCount = 0
	c.systemdJobCount = 0
	c.mu.Unlock()

	return nil
}

func (c *compositeJobSupervisor) Start() error {
	c.mu.Lock()
	hasMonit := c.monitJobCount > 0
	hasSystemd := c.systemdJobCount > 0
	c.mu.Unlock()

	if hasMonit {
		if err := c.monitSupervisor.Start(); err != nil {
			return err
		}
	}
	if hasSystemd {
		if err := c.systemdSupervisor.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (c *compositeJobSupervisor) Stop() error {
	c.mu.Lock()
	hasMonit := c.monitJobCount > 0
	hasSystemd := c.systemdJobCount > 0
	c.mu.Unlock()

	if hasMonit {
		if err := c.monitSupervisor.Stop(); err != nil {
			return err
		}
	}
	if hasSystemd {
		if err := c.systemdSupervisor.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (c *compositeJobSupervisor) StopAndWait() error {
	c.mu.Lock()
	hasMonit := c.monitJobCount > 0
	hasSystemd := c.systemdJobCount > 0
	c.mu.Unlock()

	if hasMonit {
		if err := c.monitSupervisor.StopAndWait(); err != nil {
			return err
		}
	}
	if hasSystemd {
		if err := c.systemdSupervisor.StopAndWait(); err != nil {
			return err
		}
	}
	return nil
}

func (c *compositeJobSupervisor) Unmonitor() error {
	c.mu.Lock()
	hasMonit := c.monitJobCount > 0
	hasSystemd := c.systemdJobCount > 0
	c.mu.Unlock()

	if hasMonit {
		if err := c.monitSupervisor.Unmonitor(); err != nil {
			return err
		}
	}
	if hasSystemd {
		if err := c.systemdSupervisor.Unmonitor(); err != nil {
			return err
		}
	}
	return nil
}

func (c *compositeJobSupervisor) Status() string {
	c.mu.Lock()
	hasMonit := c.monitJobCount > 0
	hasSystemd := c.systemdJobCount > 0
	c.mu.Unlock()

	if !hasMonit && !hasSystemd {
		return "running"
	}

	var monitStatus, systemdStatus string

	if hasMonit {
		monitStatus = c.monitSupervisor.Status()
	}
	if hasSystemd {
		systemdStatus = c.systemdSupervisor.Status()
	}

	if !hasMonit {
		return systemdStatus
	}
	if !hasSystemd {
		return monitStatus
	}

	// Both have jobs: aggregate statuses per the truth table in the plan
	if monitStatus == "starting" || systemdStatus == "starting" {
		return "starting"
	}
	if monitStatus == "failing" || systemdStatus == "failing" {
		return "failing"
	}
	if monitStatus == "stopped" && systemdStatus == "stopped" {
		return "stopped"
	}
	if monitStatus == "running" || systemdStatus == "running" {
		return "running"
	}

	return monitStatus
}

func (c *compositeJobSupervisor) Processes() ([]Process, error) {
	c.mu.Lock()
	hasMonit := c.monitJobCount > 0
	hasSystemd := c.systemdJobCount > 0
	c.mu.Unlock()

	var allProcesses []Process

	if hasMonit {
		monitProcesses, err := c.monitSupervisor.Processes()
		if err != nil {
			return nil, err
		}
		allProcesses = append(allProcesses, monitProcesses...)
	}

	if hasSystemd {
		systemdProcesses, err := c.systemdSupervisor.Processes()
		if err != nil {
			return nil, err
		}
		allProcesses = append(allProcesses, systemdProcesses...)
	}

	return allProcesses, nil
}

func (c *compositeJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	c.logger.Debug(compositeJobSupervisorLogTag, "Starting job failure monitoring on both delegates")

	go func() {
		if err := c.monitSupervisor.MonitorJobFailures(handler); err != nil {
			c.logger.Error(compositeJobSupervisorLogTag, "Monit failure monitor error: %s", err.Error())
		}
	}()

	return c.systemdSupervisor.MonitorJobFailures(handler)
}

func (c *compositeJobSupervisor) HealthRecorder(status string) {
}
