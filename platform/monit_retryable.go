package platform

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"

	"github.com/cloudfoundry/bosh-agent/v2/servicemanager"
)

type monitRetryable struct {
	serviceManager servicemanager.ServiceManager
}

func NewMonitRetryable(serviceManager servicemanager.ServiceManager) boshretry.Retryable {
	return &monitRetryable{
		serviceManager: serviceManager,
	}
}

func (r *monitRetryable) Attempt() (bool, error) {
	err := r.serviceManager.Start("monit")
	if err != nil {
		return true, bosherr.WrapError(err, "Starting monit")
	}

	return false, nil
}
