package app

import (
	"github.com/cloudfoundry/bosh-agent/jobsupervisor/monitor"
	"github.com/cloudfoundry/bosh-agent/platform/stats"
)

func newStatsCollector() stats.Collector {
	return monitor.NewStatsCollector()
}
