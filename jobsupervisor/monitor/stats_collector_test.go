package monitor

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
})
