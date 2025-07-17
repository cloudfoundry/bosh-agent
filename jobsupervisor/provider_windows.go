//go:build windows
// +build windows

package jobsupervisor

import (
	"os"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
	boshmonit "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/monit"
	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

const jobSupervisorListenPort = 2825

type Provider struct {
	supervisors map[string]JobSupervisor
}

func NewProvider(
	platform boshplatform.Platform,
	client boshmonit.Client,
	logger boshlog.Logger,
	dirProvider boshdir.Provider,
	handler boshhandler.Handler,
) (p Provider) {
	fs := platform.GetFs()
	runner := platform.GetRunner()

	network, err := platform.GetDefaultNetwork(boship.IPv4)
	var machineIP string
	if err != nil {
		machineIP, _ = os.Hostname() //nolint:errcheck
		logger.Debug("providerWindows", "Initializing jobsupervisor.provider_windows: %s, using hostname \"%s\"instead of IP", err, machineIP)
	} else {
		machineIP = network.IP
	}

	p.supervisors = map[string]JobSupervisor{
		"monit":      NewWrapperJobSupervisor(NewWindowsJobSupervisor(runner, dirProvider, fs, logger, jobSupervisorListenPort, make(chan bool), machineIP), fs, dirProvider, logger),
		"dummy":      NewDummyJobSupervisor(),
		"dummy-nats": NewDummyNatsJobSupervisor(handler),
		"windows":    NewWrapperJobSupervisor(NewWindowsJobSupervisor(runner, dirProvider, fs, logger, jobSupervisorListenPort, make(chan bool), machineIP), fs, dirProvider, logger),
	}

	return
}

func (p Provider) Get(name string) (supervisor JobSupervisor, err error) {
	supervisor, found := p.supervisors[name]
	if !found {
		err = bosherr.Errorf("JobSupervisor %s could not be found", name)
	}
	return
}
