package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/platform/net/arp"
)

type ForcefulARPActionArgs struct {
	Ips []string `json:"ips"`
}

type ForcefulARPAction struct {
	arp arp.Manager
}

func NewForcefulARP(arp arp.Manager) ForcefulARPAction {
	return ForcefulARPAction{
		arp: arp,
	}
}

func (a ForcefulARPAction) IsAsynchronous() bool {
	return true
}

func (a ForcefulARPAction) IsPersistent() bool {
	return false
}

func (a ForcefulARPAction) Run(args ForcefulARPActionArgs) (interface{}, error) {
	addresses := args.Ips
	for _, address := range addresses {
		a.arp.Delete(address)
	}

	resultMap := map[string]interface{}{}

	return resultMap, nil
}

func (a ForcefulARPAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ForcefulARPAction) Cancel() error {
	return errors.New("not supported")
}
