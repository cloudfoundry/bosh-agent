package fakes

import (
	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
)

type FakeMountsSearcher struct {
	SearchMountsMounts []boshdisk.Mount
	SearchMountsErr    error
}

func (s *FakeMountsSearcher) SearchMounts() ([]boshdisk.Mount, error) {
	return s.SearchMountsMounts, s.SearchMountsErr
}
