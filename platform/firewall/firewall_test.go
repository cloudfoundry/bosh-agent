package firewall_test

import (
	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IsAllowedService", func() {
	Describe("IsAllowedService", func() {
		It("returns true for ServiceMonit", func() {
			Expect(firewall.IsAllowedService(firewall.ServiceMonit)).To(BeTrue())
		})

		It("returns true for 'monit' string", func() {
			Expect(firewall.IsAllowedService(firewall.Service("monit"))).To(BeTrue())
		})

		It("returns false for unknown service", func() {
			Expect(firewall.IsAllowedService(firewall.Service("unknown"))).To(BeFalse())
		})

		It("returns false for empty service", func() {
			Expect(firewall.IsAllowedService(firewall.Service(""))).To(BeFalse())
		})

		It("returns false for similar but incorrect service names", func() {
			Expect(firewall.IsAllowedService(firewall.Service("MONIT"))).To(BeFalse())
			Expect(firewall.IsAllowedService(firewall.Service("Monit"))).To(BeFalse())
			Expect(firewall.IsAllowedService(firewall.Service("monit "))).To(BeFalse())
			Expect(firewall.IsAllowedService(firewall.Service(" monit"))).To(BeFalse())
		})
	})

	Describe("AllowedServices", func() {
		It("contains only ServiceMonit", func() {
			Expect(firewall.AllowedServices).To(ConsistOf(firewall.ServiceMonit))
		})

		It("has exactly one service", func() {
			Expect(firewall.AllowedServices).To(HaveLen(1))
		})
	})

	Describe("Constants", func() {
		It("defines correct Service value for monit", func() {
			Expect(string(firewall.ServiceMonit)).To(Equal("monit"))
		})

		It("defines cgroup versions", func() {
			Expect(firewall.CgroupV1).To(Equal(firewall.CgroupVersion(1)))
			Expect(firewall.CgroupV2).To(Equal(firewall.CgroupVersion(2)))
		})
	})

	Describe("ProcessCgroup", func() {
		It("can store cgroup v1 info", func() {
			cgroup := firewall.ProcessCgroup{
				Version: firewall.CgroupV1,
				Path:    "/system.slice/bosh-agent.service",
				ClassID: 0xb0540001,
			}
			Expect(cgroup.Version).To(Equal(firewall.CgroupV1))
			Expect(cgroup.Path).To(Equal("/system.slice/bosh-agent.service"))
			Expect(cgroup.ClassID).To(Equal(uint32(0xb0540001)))
		})

		It("can store cgroup v2 info", func() {
			cgroup := firewall.ProcessCgroup{
				Version: firewall.CgroupV2,
				Path:    "/system.slice/bosh-agent.service",
			}
			Expect(cgroup.Version).To(Equal(firewall.CgroupV2))
			Expect(cgroup.Path).To(Equal("/system.slice/bosh-agent.service"))
			Expect(cgroup.ClassID).To(Equal(uint32(0))) // Not used in v2
		})
	})
})
