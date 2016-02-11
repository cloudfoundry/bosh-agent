package jobsupervisor

type windowsJobSupervisor struct {
	processes []Process
	status    string
}

func NewWindowsJobSupervisor() JobSupervisor {
	return &windowsJobSupervisor{status: "unmonitored"}
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
	return nil
}

func (s *windowsJobSupervisor) RemoveAllJobs() error {
	return nil
}

func (s *windowsJobSupervisor) MonitorJobFailures(handler JobFailureHandler) error {
	return nil
}
