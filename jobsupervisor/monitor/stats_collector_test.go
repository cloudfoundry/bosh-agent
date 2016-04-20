// +build windows

package monitor

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/stats"
)

var _ = Describe("Stats collector", func() {
	Context("when calculating CPU usage", func() {
		It("should correctly format it for usage by stats.CPUStats", func() {
			m := &Monitor{
				user:   CPUTime{load: 0.25},
				kernel: CPUTime{load: 0.50},
				idle:   CPUTime{load: 0.00},
			}
			c := collector{m: m}
			cpu, err := c.GetCPUStats()
			Expect(err).To(HaveOccurred())
			Expect(matchFloat(cpu.UserPercent().FractionOf100(), m.user.load*100)).To(Succeed())
			Expect(matchFloat(cpu.SysPercent().FractionOf100(), m.kernel.load*100)).To(Succeed())
		})
	})

	It("should start collecting when StartCollecting is called", func() {
		const freq = time.Second

		sema := make(chan struct{})
		c := NewStatsCollector()
		c.StartCollecting(freq, sema)

		<-sema
		cpu, err := c.GetCPUStats()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpu).ToNot(Equal(stats.CPUStats{}))

		Eventually(func() (stats.CPUStats, error) {
			<-sema
			return c.GetCPUStats()
		}, freq*10).ShouldNot(Equal(cpu))
	})

	It("should handle a nil sema channel", func() {
		const freq = time.Second

		c := NewStatsCollector()
		c.StartCollecting(freq, nil)

		cpu, err := c.GetCPUStats()
		Expect(err).ToNot(HaveOccurred())
		Expect(cpu).ToNot(Equal(stats.CPUStats{}))
	})
})
