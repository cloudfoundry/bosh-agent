package vitals

import (
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshstats "github.com/cloudfoundry/bosh-agent/platform/stats"
)

type concreteService struct {
	statsCollector boshstats.Collector
	dirProvider    boshdirs.Provider
}

func NewService(statsCollector boshstats.Collector, dirProvider boshdirs.Provider) Service {
  return concreteService{
    statsCollector: statsCollector,
    dirProvider:    dirProvider,
  }
}

func createLoadVitals(loadStats boshstats.CPULoad) []string {
	return []string{""}
}

func (s concreteService) isMountPoint(dir string) bool { return true }
