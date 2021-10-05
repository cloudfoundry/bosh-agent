package vitals_test

import (
	"path"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/disk/diskfakes"
	boshstats "github.com/cloudfoundry/bosh-agent/platform/stats"
	fakestats "github.com/cloudfoundry/bosh-agent/platform/stats/fakes"
	. "github.com/cloudfoundry/bosh-agent/platform/vitals"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
)

const Windows = runtime.GOOS == "windows"

var _ = Describe("Vitals service", func() {
	var (
		dirProvider    boshdirs.Provider
		statsCollector *fakestats.FakeCollector
		mounter        *diskfakes.FakeMounter
		service        Service
	)

	BeforeEach(func() {
		dirProvider = boshdirs.NewProvider("/fake/base/dir")
		statsCollector = &fakestats.FakeCollector{
			CPULoad: boshstats.CPULoad{
				One:     0.2,
				Five:    4.55,
				Fifteen: 1.123,
			},
			StartCollectingCPUStats: boshstats.CPUStats{
				User:  56,
				Sys:   10,
				Wait:  1,
				Total: 100,
			},
			MemStats: boshstats.Usage{
				Used:  700 * 1024,
				Total: 1000 * 1024,
			},
			SwapStats: boshstats.Usage{
				Used:  600 * 1024,
				Total: 1000 * 1024,
			},
			UptimeStats: boshstats.UptimeStats{
				Secs: 5,
			},
			DiskStats: map[string]boshstats.DiskStats{
				"/": {
					DiskUsage:  boshstats.Usage{Used: 100, Total: 200},
					InodeUsage: boshstats.Usage{Used: 50, Total: 500},
				},
				dirProvider.DataDir(): {
					DiskUsage:  boshstats.Usage{Used: 15, Total: 20},
					InodeUsage: boshstats.Usage{Used: 10, Total: 50},
				},
				dirProvider.StoreDir(): {
					DiskUsage:  boshstats.Usage{Used: 2, Total: 2},
					InodeUsage: boshstats.Usage{Used: 3, Total: 4},
				},
			},
		}

		mounter = &diskfakes.FakeMounter{}
		mounter.IsMountPointReturns("/dev/fake-partition-device", true, nil)

		service = NewService(statsCollector, dirProvider, mounter)
		statsCollector.StartCollecting(1*time.Millisecond, nil)
	})

	It("constructs vitals properly", func() {
		vitals, err := service.Get()

		expectedVitals := map[string]interface{}{
			"cpu": map[string]string{
				"sys":  "10.0",
				"user": "56.0",
				"wait": "1.0",
			},
			"disk": map[string]interface{}{
				"system": map[string]string{
					"percent":       "50",
					"inode_percent": "10",
				},
				"ephemeral": map[string]string{
					"percent":       "75",
					"inode_percent": "20",
				},
				"persistent": map[string]string{
					"percent":       "100",
					"inode_percent": "75",
				},
			},
			"mem": map[string]string{
				"kb":      "700",
				"percent": "70",
			},
			"swap": map[string]string{
				"kb":      "600",
				"percent": "60",
			},
			"uptime": map[string]uint64{
				"secs": 5,
			},
		}
		if Windows {
			expectedVitals["load"] = []string{""}
		} else {
			expectedVitals["load"] = []string{"0.20", "4.55", "1.12"}
		}

		Expect(err).ToNot(HaveOccurred())

		Expect(mounter.IsMountPointCallCount()).To(Equal(3))

		boshassert.MatchesJSONMap(GinkgoT(), vitals, expectedVitals)
	})

	Context("when missing stats for ephemeral and peristent disk", func() {
		BeforeEach(func() {
			statsCollector.DiskStats = map[string]boshstats.DiskStats{
				"/": {
					DiskUsage:  boshstats.Usage{Used: 100, Total: 200},
					InodeUsage: boshstats.Usage{Used: 50, Total: 500},
				},
			}
		})

		It("returns vitals for root disk only", func() {
			vitals, err := service.Get()
			Expect(err).ToNot(HaveOccurred())

			boshassert.LacksJSONKey(GinkgoT(), vitals.Disk, "ephemeral")
			boshassert.LacksJSONKey(GinkgoT(), vitals.Disk, "persistent")
		})
	})

	Context("when missing stats for system disk", func() {
		BeforeEach(func() {
			statsCollector.DiskStats = map[string]boshstats.DiskStats{}
		})

		It("returns an error", func() {
			_, err := service.Get()
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when no persistent disk is mounted", func() {
		BeforeEach(func() {
			mounter.IsMountPointStub = func(folderPath string) (partitionPath string, isMountPoint bool, err error) {
				partitionPath = "/dev/fake-partition-device"
				isMountPoint = path.Base(folderPath) != "store"
				return
			}
		})

		It("does not return vitals for persistent disk", func() {
			vitals, err := service.Get()

			Expect(err).NotTo(HaveOccurred())
			boshassert.LacksJSONKey(GinkgoT(), vitals.Disk, "persistent")
		})
	})
})
