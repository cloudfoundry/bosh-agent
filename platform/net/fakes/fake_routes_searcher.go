package fakes

import (
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	"github.com/coreos/go-iptables/iptables"
)

type FakeRoutesSearcher struct {
	SearchRoutesRoutes []boshnet.Route
	SearchRoutesErr    error
}

func (s *FakeRoutesSearcher) SearchRoutes(ipProtocol iptables.Protocol) ([]boshnet.Route, error) {
	return s.SearchRoutesRoutes, s.SearchRoutesErr
}
