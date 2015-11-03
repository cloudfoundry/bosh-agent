package action

import (
	"errors"
	"path/filepath"

	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type StopAction struct {
	jobSupervisor boshjobsuper.JobSupervisor
	fs            boshsys.FileSystem
	dirProvider   boshdirs.Provider
}

func NewStop(jobSupervisor boshjobsuper.JobSupervisor, fs boshsys.FileSystem, dirProvider boshdirs.Provider) (stop StopAction) {
	stop = StopAction{
		jobSupervisor: jobSupervisor,
		fs:            fs,
		dirProvider:   dirProvider,
	}
	return
}

func (a StopAction) IsAsynchronous() bool {
	return true
}

func (a StopAction) IsPersistent() bool {
	return false
}

func (a StopAction) Run() (value string, err error) {
	err = a.jobSupervisor.Stop()
	if err != nil {
		err = bosherr.WrapError(err, "Stopping Monitored Services")
		return
	}

	stoppedFile := filepath.Join(a.dirProvider.MonitDir(), "stopped")
	err = a.fs.WriteFileString(stoppedFile, "")
	if err != nil {
		err = bosherr.WrapError(err, "Writing Stopped File")
		return
	}

	value = "stopped"
	return
}

func (a StopAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a StopAction) Cancel() error {
	return errors.New("not supported")
}
