package fakes

import (
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
)

type FakeRoutesSearcher struct {
	SearchRoutesRoutes []boshnet.Route
	SearchRoutesErr    error
}

func (s *FakeRoutesSearcher) SearchRoutes(ipv6 bool) ([]boshnet.Route, error) {
	return s.SearchRoutesRoutes, s.SearchRoutesErr
}
