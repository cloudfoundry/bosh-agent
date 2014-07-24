package applyspec

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type DefaultNetworkDelegate interface {
	GetDefaultNetwork() (boshsettings.Network, error)
}
