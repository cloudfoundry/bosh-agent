package jobsupervisor

type windowsJobSupervisor struct {
	processes []Process
}

func NewWindowsJobSupervisor() JobSupervisor {
	return &windowsJobSupervisor{}
}

func (s *windowsJobSupervisor) Reload() error {
	return nil
}

func (s *windowsJobSupervisor) Start() error {
	s.processes = []Process{}
	return nil
}

func (s *windowsJobSupervisor) Stop() error {
	return nil
}

func (s *windowsJobSupervisor) Unmonitor() error {
	return nil
}

func (s *windowsJobSupervisor) Status() (status string) {
	return "running"
}

func (s *windowsJobSupervisor) Processes() ([]Process, error) {
	return s.processes, nil
}

func (s *windowsJobSupervisor) AddJob(jobName string, jobIndex int, configPath string) error {
	return nil
}

func (s *windowsJobSupervisor) RemoveAllJobs() error {
	return nil
}

func (s *windowsJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	return nil
}
