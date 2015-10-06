package jobsupervisor

import (
	"time"

	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	boshmonit "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

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
	monitJobSupervisor := NewMonitJobSupervisor(
		platform.GetFs(),
		platform.GetRunner(),
		client,
		logger,
		dirProvider,
		2825,
		MonitReloadOptions{
			MaxTries:               3,
			MaxCheckTries:          6,
			DelayBetweenCheckTries: 5 * time.Second,
		},
	)

	p.supervisors = map[string]JobSupervisor{
		"monit":      monitJobSupervisor,
		"dummy":      NewDummyJobSupervisor(),
		"dummy-nats": NewDummyNatsJobSupervisor(handler),
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
