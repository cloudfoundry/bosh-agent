package action

import (
	"errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
)

type ForcefulARPActionArgs struct {
	Ips []string `json:"ips"`
}

type ForcefulARPAction struct {
	platform boshplatform.Platform
}

func NewForcefulARP(platform boshplatform.Platform) ForcefulARPAction {
	return ForcefulARPAction{
		platform: platform,
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
		a.platform.DeleteArpEntryWithIp(address)
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
