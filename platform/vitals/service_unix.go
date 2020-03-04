// +build !windows

package vitals

import (
	"fmt"

	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
	boshstats "github.com/cloudfoundry/bosh-agent/platform/stats"
)

type concreteService struct {
	statsCollector boshstats.Collector
	dirProvider    boshdirs.Provider
	mounter        boshdisk.Mounter
}

func NewService(statsCollector boshstats.Collector, dirProvider boshdirs.Provider, mounter boshdisk.Mounter) Service {
  return concreteService{
    statsCollector: statsCollector,
    dirProvider:    dirProvider,
		mounter:        mounter,
  }
}

func createLoadVitals(loadStats boshstats.CPULoad) []string {
	return []string{
		fmt.Sprintf("%.2f", loadStats.One),
		fmt.Sprintf("%.2f", loadStats.Five),
		fmt.Sprintf("%.2f", loadStats.Fifteen),
	}
}

func (s concreteService) isMountPoint(dir string) bool {
	_, found, err := s.mounter.IsMountPoint(dir)

	if err != nil {
		return false
	}

	return found
}
