// +build !windows

package app

import (
	"github.com/cloudfoundry/bosh-agent/platform/stats"
	boshsigar "github.com/cloudfoundry/bosh-agent/sigar"
	sigar "github.com/cloudfoundry/gosigar"
)

func newStatsCollector() stats.Collector {
	return boshsigar.NewSigarStatsCollector(&sigar.ConcreteSigar{})
}
