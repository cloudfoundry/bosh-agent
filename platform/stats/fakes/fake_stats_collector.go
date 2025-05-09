package fakes

import (
	"errors"
	"time"

	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
)

type FakeCollector struct {
	StartCollectingCPUStats boshstats.CPUStats

	CPULoad  boshstats.CPULoad
	cpuStats boshstats.CPUStats

	MemStats    boshstats.Usage
	MemStatsErr error

	SwapStats boshstats.Usage
	DiskStats map[string]boshstats.DiskStats

	UptimeStats boshstats.UptimeStats
}

func (c *FakeCollector) StartCollecting(collectionInterval time.Duration, latestGotUpdated chan struct{}) {
	c.cpuStats = c.StartCollectingCPUStats
}

func (c *FakeCollector) GetCPULoad() (load boshstats.CPULoad, err error) {
	load = c.CPULoad
	return
}

func (c *FakeCollector) GetCPUStats() (stats boshstats.CPUStats, err error) {
	stats = c.cpuStats
	return
}

func (c *FakeCollector) GetMemStats() (boshstats.Usage, error) {
	return c.MemStats, c.MemStatsErr
}

func (c *FakeCollector) GetSwapStats() (usage boshstats.Usage, err error) {
	usage = c.SwapStats
	return
}

func (c *FakeCollector) GetDiskStats(devicePath string) (stats boshstats.DiskStats, err error) {
	stats, found := c.DiskStats[devicePath]
	if !found {
		err = errors.New("Disk not found") //nolint:staticcheck
	}
	return
}

func (c *FakeCollector) GetUptimeStats() (stats boshstats.UptimeStats, err error) {
	stats = c.UptimeStats
	return
}
