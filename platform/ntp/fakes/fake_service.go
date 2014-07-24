package fakes

import boshntp "github.com/cloudfoundry/bosh-agent/platform/ntp"

type FakeService struct {
	GetOffsetNTPOffset boshntp.NTPInfo
}

func (oc *FakeService) GetInfo() (ntpInfo boshntp.NTPInfo) {
	ntpInfo = oc.GetOffsetNTPOffset
	return
}
