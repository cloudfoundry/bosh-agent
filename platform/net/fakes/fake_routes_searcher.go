package fakes

import (
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

type FakeRoutesSearcher struct {
	SearchRoutesRoutes []boshnet.Route
	SearchRoutesErr    error
}

func (s *FakeRoutesSearcher) SearchRoutes(ipProtocol boship.IPProtocol) ([]boshnet.Route, error) {
	return s.SearchRoutesRoutes, s.SearchRoutesErr
}
