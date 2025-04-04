package vitals

import (
	"fmt"

	sigar "github.com/cloudfoundry/gosigar"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"

	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Service

type Service interface {
	Get() (vitals Vitals, err error)
}

type concreteService struct {
	statsCollector boshstats.Collector
	dirProvider    boshdirs.Provider
	diskMounter    boshdisk.Mounter
}

func NewService(
	statsCollector boshstats.Collector,
	dirProvider boshdirs.Provider,
	diskMounter boshdisk.Mounter,
) Service {
	return concreteService{
		statsCollector: statsCollector,
		dirProvider:    dirProvider,
		diskMounter:    diskMounter,
	}
}

func (s concreteService) Get() (Vitals, error) {
	var (
		loadStats   boshstats.CPULoad
		cpuStats    boshstats.CPUStats
		memStats    boshstats.Usage
		swapStats   boshstats.Usage
		uptimeStats boshstats.UptimeStats
		diskStats   DiskVitals
	)

	vitals := Vitals{}

	loadStats, err := s.statsCollector.GetCPULoad()
	if err != nil && err != sigar.ErrNotImplemented {
		return vitals, bosherr.WrapError(err, "Getting CPU Load")
	}

	cpuStats, err = s.statsCollector.GetCPUStats()
	if err != nil {
		return vitals, bosherr.WrapError(err, "Getting CPU Stats")
	}

	memStats, err = s.statsCollector.GetMemStats()
	if err != nil {
		return vitals, bosherr.WrapError(err, "Getting Memory Stats")
	}

	swapStats, err = s.statsCollector.GetSwapStats()
	if err != nil {
		return vitals, bosherr.WrapError(err, "Getting Swap Stats")
	}

	diskStats, err = s.getDiskStats()
	if err != nil {
		return vitals, bosherr.WrapError(err, "Getting Disk Stats")
	}

	uptimeStats, err = s.statsCollector.GetUptimeStats()
	if err != nil {
		return vitals, bosherr.WrapError(err, "Getting Uptime Stats")
	}

	return Vitals{
		Load: createLoadVitals(loadStats),
		CPU: CPUVitals{
			User: cpuStats.UserPercent().FormatFractionOf100(1),
			Sys:  cpuStats.SysPercent().FormatFractionOf100(1),
			Wait: cpuStats.WaitPercent().FormatFractionOf100(1),
		},
		Mem:    createMemVitals(memStats),
		Swap:   createMemVitals(swapStats),
		Disk:   diskStats,
		Uptime: UptimeVitals{Secs: uptimeStats.Secs},
	}, nil
}

func (s concreteService) getDiskStats() (DiskVitals, error) {
	disks := map[string]string{
		"/":                      "system",
		s.dirProvider.DataDir():  "ephemeral",
		s.dirProvider.StoreDir(): "persistent",
	}
	diskStats := make(DiskVitals, len(disks))

	for path, name := range disks {
		diskStats, err := s.addDiskStats(diskStats, path, name)
		if err != nil {
			return diskStats, err
		}
	}

	return diskStats, nil
}

func (s concreteService) addDiskStats(diskStats DiskVitals, path, name string) (DiskVitals, error) {
	if s.diskMounter != nil {
		var isMountPoint bool
		_, isMountPoint, err := s.diskMounter.IsMountPoint(path)
		if err != nil {
			return diskStats, bosherr.WrapError(err, fmt.Sprintf("Verifying if '%s' is a mount point", path))
		}
		if !isMountPoint {
			return diskStats, nil
		}
	}

	stat, diskErr := s.statsCollector.GetDiskStats(path)
	if diskErr != nil {
		if path == "/" {
			return diskStats, bosherr.WrapError(diskErr, "Getting Disk Stats for /")
		}
		return diskStats, nil
	}

	diskStats[name] = SpecificDiskVitals{
		Percent:      stat.DiskUsage.Percent().FormatFractionOf100(0),
		InodePercent: stat.InodeUsage.Percent().FormatFractionOf100(0),
	}
	return diskStats, nil
}

func createMemVitals(memUsage boshstats.Usage) MemoryVitals {
	return MemoryVitals{
		Percent: memUsage.Percent().FormatFractionOf100(0),
		Kb:      fmt.Sprintf("%d", memUsage.Used/1024),
	}
}
