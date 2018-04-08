package fakes

import (
	boshnet "github.com/cloudfoundry/bosh-agent/platform/net"
)

type FakeInterfaceManager struct {
	GetInterfacesInterfaces []boshnet.Interface
	GetInterfacesErr        error
}

func (i *FakeInterfaceManager) GetInterfaces() ([]boshnet.Interface, error) {
	return i.GetInterfacesInterfaces, i.GetInterfacesErr
}
