//go:build linux

package firewall_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall"
	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall/firewallfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NftablesFirewall", func() {
	var (
		fakeConn           *firewallfakes.FakeNftablesConn
		fakeCgroupResolver *firewallfakes.FakeCgroupResolver
		logger             boshlog.Logger
	)

	BeforeEach(func() {
		fakeConn = new(firewallfakes.FakeNftablesConn)
		fakeCgroupResolver = new(firewallfakes.FakeCgroupResolver)
		logger = boshlog.NewLogger(boshlog.LevelNone)

		// Default successful returns
		fakeCgroupResolver.DetectVersionReturns(firewall.CgroupV2, nil)
		fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
			Version: firewall.CgroupV2,
			Path:    "/system.slice/bosh-agent.service",
		}, nil)
		fakeConn.FlushReturns(nil)
	})

	Describe("NewNftablesFirewallWithDeps", func() {
		It("creates a firewall manager with cgroup v2", func() {
			fakeCgroupResolver.DetectVersionReturns(firewall.CgroupV2, nil)

			mgr, err := firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(mgr).ToNot(BeNil())
			Expect(fakeCgroupResolver.DetectVersionCallCount()).To(Equal(1))
		})

		It("creates a firewall manager with cgroup v1", func() {
			fakeCgroupResolver.DetectVersionReturns(firewall.CgroupV1, nil)

			mgr, err := firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).ToNot(HaveOccurred())
			Expect(mgr).ToNot(BeNil())
		})

		It("returns error when cgroup detection fails", func() {
			fakeCgroupResolver.DetectVersionReturns(firewall.CgroupV1, errors.New("cgroup detection failed"))

			_, err := firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Detecting cgroup version"))
		})
	})

	Describe("SetupAgentRules", func() {
		var mgr firewall.Manager

		BeforeEach(func() {
			var err error
			mgr, err = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates table and monit chain", func() {
			err := mgr.SetupAgentRules("nats://user:pass@10.0.0.1:4222", true)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.AddTableCallCount()).To(Equal(1))
			table := fakeConn.AddTableArgsForCall(0)
			Expect(table.Name).To(Equal(firewall.TableName))
			Expect(table.Family).To(Equal(nftables.TableFamilyINet))

			// When enableNATSFirewall is true, both monit and NATS chains are created
			Expect(fakeConn.AddChainCallCount()).To(Equal(2))
			monitChain := fakeConn.AddChainArgsForCall(0)
			Expect(monitChain.Name).To(Equal(firewall.MonitChainName))
			Expect(monitChain.Type).To(Equal(nftables.ChainTypeFilter))
			Expect(monitChain.Hooknum).To(Equal(nftables.ChainHookOutput))

			natsChain := fakeConn.AddChainArgsForCall(1)
			Expect(natsChain.Name).To(Equal(firewall.NATSChainName))
		})

		It("adds monit rule", func() {
			err := mgr.SetupAgentRules("", false)
			Expect(err).ToNot(HaveOccurred())

			// At least one rule should be added (monit rule)
			Expect(fakeConn.AddRuleCallCount()).To(BeNumerically(">=", 1))
		})

		It("flushes rules after adding", func() {
			err := mgr.SetupAgentRules("", false)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.FlushCallCount()).To(Equal(1))
		})

		It("returns error when flush fails", func() {
			fakeConn.FlushReturns(errors.New("flush failed"))

			err := mgr.SetupAgentRules("", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Flushing nftables rules"))
		})

		It("returns error when getting process cgroup fails", func() {
			fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{}, errors.New("cgroup error"))

			err := mgr.SetupAgentRules("", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Getting agent cgroup"))
		})

		Context("when enableNATSFirewall is true with NATS URL", func() {
			It("creates NATS chain but does not add NATS rules (rules added via BeforeConnect)", func() {
				err := mgr.SetupAgentRules("nats://user:pass@10.0.0.1:4222", true)
				Expect(err).ToNot(HaveOccurred())

				// Should have monit rule only - NATS rules are added via BeforeConnect
				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				// Both chains should be created
				Expect(fakeConn.AddChainCallCount()).To(Equal(2))
			})

			It("skips NATS chain creation for empty URL", func() {
				err := mgr.SetupAgentRules("", true)
				Expect(err).ToNot(HaveOccurred())

				// Both chains created, only monit rule
				Expect(fakeConn.AddChainCallCount()).To(Equal(2))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
			})

			It("skips NATS chain creation for https:// URL (create-env case)", func() {
				err := mgr.SetupAgentRules("https://mbus.bosh-lite.com:6868", true)
				Expect(err).ToNot(HaveOccurred())

				// Both chains created, only monit rule
				Expect(fakeConn.AddChainCallCount()).To(Equal(2))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
			})
		})

		Context("when enableNATSFirewall is false", func() {
			It("only creates monit chain, no NATS chain", func() {
				err := mgr.SetupAgentRules("nats://user:pass@10.0.0.1:4222", false)
				Expect(err).ToNot(HaveOccurred())

				// Should only create monit chain (no NATS chain)
				Expect(fakeConn.AddChainCallCount()).To(Equal(1))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
			})

			It("adds monit rule", func() {
				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
			})
		})

		Context("when cgroup version is v2", func() {
			BeforeEach(func() {
				fakeCgroupResolver.DetectVersionReturns(firewall.CgroupV2, nil)
				fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
					Version: firewall.CgroupV2,
					Path:    "/system.slice/bosh-agent.service",
				}, nil)
				var err error
				mgr, err = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates rule with cgroup v2 path in expressions", func() {
				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)

				// Verify the rule contains a Socket expression for cgroupv2
				var hasSocketExpr bool
				var hasCgroupPath bool
				for _, e := range rule.Exprs {
					if socketExpr, ok := e.(*expr.Socket); ok {
						Expect(socketExpr.Key).To(Equal(expr.SocketKeyCgroupv2))
						hasSocketExpr = true
					}
					if cmpExpr, ok := e.(*expr.Cmp); ok {
						// Check if the cgroup path is in the comparison data
						if string(cmpExpr.Data) == "/system.slice/bosh-agent.service\x00" {
							hasCgroupPath = true
						}
					}
				}
				Expect(hasSocketExpr).To(BeTrue(), "Expected Socket expression for cgroupv2")
				Expect(hasCgroupPath).To(BeTrue(), "Expected cgroup path in Cmp expression")
			})

			It("creates rule with nested container cgroup path", func() {
				nestedPath := "/docker/abc123def456/system.slice/bosh-agent.service"
				fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
					Version: firewall.CgroupV2,
					Path:    nestedPath,
				}, nil)

				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)

				// Verify the nested cgroup path is in the rule
				var foundNestedPath bool
				for _, e := range rule.Exprs {
					if cmpExpr, ok := e.(*expr.Cmp); ok {
						if string(cmpExpr.Data) == nestedPath+"\x00" {
							foundNestedPath = true
						}
					}
				}
				Expect(foundNestedPath).To(BeTrue(), "Expected nested cgroup path '%s' in rule expressions", nestedPath)
			})

			It("creates rule with deeply nested cgroup path (multiple container levels)", func() {
				deeplyNestedPath := "/docker/outer123/docker/inner456/system.slice/bosh-agent.service"
				fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
					Version: firewall.CgroupV2,
					Path:    deeplyNestedPath,
				}, nil)

				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)

				var foundPath bool
				for _, e := range rule.Exprs {
					if cmpExpr, ok := e.(*expr.Cmp); ok {
						if string(cmpExpr.Data) == deeplyNestedPath+"\x00" {
							foundPath = true
						}
					}
				}
				Expect(foundPath).To(BeTrue(), "Expected deeply nested cgroup path in rule expressions")
			})

			It("creates rule with root cgroup path", func() {
				fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
					Version: firewall.CgroupV2,
					Path:    "/",
				}, nil)

				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)

				var foundRootPath bool
				for _, e := range rule.Exprs {
					if cmpExpr, ok := e.(*expr.Cmp); ok {
						if string(cmpExpr.Data) == "/\x00" {
							foundRootPath = true
						}
					}
				}
				Expect(foundRootPath).To(BeTrue(), "Expected root cgroup path in rule expressions")
			})
		})

		Context("when cgroup version is v1", func() {
			BeforeEach(func() {
				fakeCgroupResolver.DetectVersionReturns(firewall.CgroupV1, nil)
				fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
					Version: firewall.CgroupV1,
					Path:    "/system.slice/bosh-agent.service",
					ClassID: firewall.MonitClassID,
				}, nil)
				var err error
				mgr, err = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
				Expect(err).ToNot(HaveOccurred())
			})

			It("creates rule with cgroup v1 classid in expressions", func() {
				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)

				// Verify the rule contains a Meta expression for cgroup classid
				var hasMetaExpr bool
				for _, e := range rule.Exprs {
					if metaExpr, ok := e.(*expr.Meta); ok {
						if metaExpr.Key == expr.MetaKeyCGROUP {
							hasMetaExpr = true
						}
					}
				}
				Expect(hasMetaExpr).To(BeTrue(), "Expected Meta CGROUP expression for cgroup v1")
			})

			It("creates rule with container cgroup classid", func() {
				fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{
					Version: firewall.CgroupV1,
					Path:    "/docker/abc123def456",
					ClassID: firewall.MonitClassID,
				}, nil)

				err := mgr.SetupAgentRules("", false)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)

				// Verify Meta CGROUP expression exists
				var hasMetaExpr bool
				for _, e := range rule.Exprs {
					if metaExpr, ok := e.(*expr.Meta); ok {
						if metaExpr.Key == expr.MetaKeyCGROUP {
							hasMetaExpr = true
						}
					}
				}
				Expect(hasMetaExpr).To(BeTrue(), "Expected Meta CGROUP expression for container cgroup")
			})
		})
	})

	Describe("AllowService", func() {
		var mgr firewall.Manager

		BeforeEach(func() {
			var err error
			mgr, err = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows monit service", func() {
			err := mgr.AllowService(firewall.ServiceMonit, 1234)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.AddTableCallCount()).To(Equal(1))
			Expect(fakeConn.AddChainCallCount()).To(Equal(1))
			Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
			Expect(fakeConn.FlushCallCount()).To(Equal(1))
		})

		It("looks up cgroup for caller PID", func() {
			err := mgr.AllowService(firewall.ServiceMonit, 5678)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCgroupResolver.GetProcessCgroupCallCount()).To(Equal(1))
			pid, version := fakeCgroupResolver.GetProcessCgroupArgsForCall(0)
			Expect(pid).To(Equal(5678))
			Expect(version).To(Equal(firewall.CgroupV2))
		})

		It("rejects unknown service", func() {
			err := mgr.AllowService(firewall.Service("unknown"), 1234)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not in allowed list"))

			// Should not add any rules
			Expect(fakeConn.AddRuleCallCount()).To(Equal(0))
		})

		It("returns error when getting caller cgroup fails", func() {
			fakeCgroupResolver.GetProcessCgroupReturns(firewall.ProcessCgroup{}, errors.New("no such process"))

			err := mgr.AllowService(firewall.ServiceMonit, 99999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Getting caller cgroup"))
		})

		It("returns error when flush fails", func() {
			fakeConn.FlushReturns(errors.New("flush failed"))

			err := mgr.AllowService(firewall.ServiceMonit, 1234)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Flushing nftables rules"))
		})
	})

	Describe("Cleanup", func() {
		var mgr firewall.Manager

		BeforeEach(func() {
			var err error
			mgr, err = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		It("deletes table and flushes after SetupAgentRules", func() {
			// First set up rules to create the table
			err := mgr.SetupAgentRules("", false)
			Expect(err).ToNot(HaveOccurred())

			// Now cleanup
			err = mgr.Cleanup()
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.DelTableCallCount()).To(Equal(1))
			// Flush is called during setup and cleanup
			Expect(fakeConn.FlushCallCount()).To(Equal(2))
		})

		It("does not delete table if never set up", func() {
			err := mgr.Cleanup()
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.DelTableCallCount()).To(Equal(0))
			Expect(fakeConn.FlushCallCount()).To(Equal(1))
		})
	})

	Describe("BeforeConnect", func() {
		var mgr firewall.Manager

		BeforeEach(func() {
			var err error
			mgr, err = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeCgroupResolver, logger)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when NATS firewall is enabled", func() {
			BeforeEach(func() {
				// First set up agent rules with NATS firewall enabled
				err := mgr.SetupAgentRules("nats://user:pass@10.0.0.1:4222", true)
				Expect(err).ToNot(HaveOccurred())
			})

			It("adds NATS rule for IP address", func() {
				hook := mgr.(firewall.NatsFirewallHook)
				err := hook.BeforeConnect("nats://user:pass@10.0.0.1:4222")
				Expect(err).ToNot(HaveOccurred())

				// Should flush NATS chain and add new rules
				Expect(fakeConn.FlushChainCallCount()).To(Equal(1))
				// 1 monit rule from setup + 2 NATS rules (ACCEPT + DROP) from BeforeConnect
				Expect(fakeConn.AddRuleCallCount()).To(Equal(3))
			})

			It("adds NATS rule for IPv6 address", func() {
				hook := mgr.(firewall.NatsFirewallHook)
				err := hook.BeforeConnect("nats://user:pass@[::1]:4222")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.FlushChainCallCount()).To(Equal(1))
				// 1 monit rule from setup + 2 NATS rules (ACCEPT + DROP) from BeforeConnect
				Expect(fakeConn.AddRuleCallCount()).To(Equal(3))
			})

			It("skips for https:// URL (create-env case)", func() {
				hook := mgr.(firewall.NatsFirewallHook)
				err := hook.BeforeConnect("https://mbus.bosh-lite.com:6868")
				Expect(err).ToNot(HaveOccurred())

				// No flush or additional rules
				Expect(fakeConn.FlushChainCallCount()).To(Equal(0))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(1)) // Only monit from setup
			})

			It("skips for empty URL", func() {
				hook := mgr.(firewall.NatsFirewallHook)
				err := hook.BeforeConnect("")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeConn.FlushChainCallCount()).To(Equal(0))
			})
		})

		Context("when NATS firewall is disabled", func() {
			BeforeEach(func() {
				err := mgr.SetupAgentRules("nats://user:pass@10.0.0.1:4222", false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does nothing", func() {
				hook := mgr.(firewall.NatsFirewallHook)
				err := hook.BeforeConnect("nats://user:pass@10.0.0.1:4222")
				Expect(err).ToNot(HaveOccurred())

				// No flush, no additional rules
				Expect(fakeConn.FlushChainCallCount()).To(Equal(0))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(1)) // Only monit from setup
			})
		})
	})

	Describe("Constants", func() {
		It("defines MonitClassID correctly", func() {
			// MonitClassID should be 0xb0540001 = 2958295041
			// This is "b054" (BOSH leet) in the major number, 0001 in minor
			Expect(firewall.MonitClassID).To(Equal(uint32(0xb0540001)))
			Expect(firewall.MonitClassID).To(Equal(uint32(2958295041)))
		})

		It("defines NATSClassID correctly", func() {
			// NATSClassID should be 0xb0540002 = 2958295042
			Expect(firewall.NATSClassID).To(Equal(uint32(0xb0540002)))
			Expect(firewall.NATSClassID).To(Equal(uint32(2958295042)))
		})

		It("defines different classids for monit and nats", func() {
			Expect(firewall.MonitClassID).ToNot(Equal(firewall.NATSClassID))
		})

		It("defines table and chain names", func() {
			Expect(firewall.TableName).To(Equal("bosh_agent"))
			Expect(firewall.MonitChainName).To(Equal("monit_access"))
			Expect(firewall.NATSChainName).To(Equal("nats_access"))
		})

		It("defines monit port", func() {
			Expect(firewall.MonitPort).To(Equal(2822))
		})
	})
})
