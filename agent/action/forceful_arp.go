package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/platform/net/arp"
)

type ForcefulARPAction struct {
	arp arp.Manager
}

func NewForcefulARP(arp arp.Manager) ForcefulARPAction {
	return ForcefulARPAction{
		arp: arp,
	}
}

func (a ForcefulARPAction) IsAsynchronous() bool {
	return false
}

func (a ForcefulARPAction) IsPersistent() bool {
	return false
}

func (a ForcefulARPAction) Run(addresses []string) (string, error) {
	for _, address := range addresses {
		a.arp.Delete(address)
	}
	return "completed", nil
}

func (a ForcefulARPAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ForcefulARPAction) Cancel() error {
	return errors.New("not supported")
}
