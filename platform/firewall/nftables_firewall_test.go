//go:build linux

package firewall_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"

	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall"
	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall/firewallfakes"
)

var _ = Describe("NftablesFirewall", func() {
	var (
		fakeConn     *firewallfakes.FakeNftablesConn
		fakeResolver *firewallfakes.FakeDNSResolver
		logger       boshlog.Logger
		manager      firewall.Manager
	)

	BeforeEach(func() {
		fakeConn = &firewallfakes.FakeNftablesConn{}
		fakeResolver = &firewallfakes.FakeDNSResolver{}
		logger = boshlog.NewWriterLogger(boshlog.LevelDebug, GinkgoWriter)
		manager = firewall.NewNftablesFirewallWithDeps(fakeConn, fakeResolver, logger)
	})

	Describe("SetupMonitFirewall", func() {
		It("creates table, chains, and rules successfully", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConn.AddTableCallCount()).To(Equal(1))
			Expect(fakeConn.AddChainCallCount()).To(Equal(2)) // jobs chain + monit chain
			Expect(fakeConn.FlushChainCallCount()).To(Equal(1))
			Expect(fakeConn.AddRuleCallCount()).To(Equal(3)) // jump + allow + block
			Expect(fakeConn.FlushCallCount()).To(Equal(1))
		})

		It("creates table with correct configuration", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			table := fakeConn.AddTableArgsForCall(0)
			Expect(table.Name).To(Equal("bosh_agent"))
			Expect(table.Family).To(Equal(nftables.TableFamilyINet))
		})

		It("creates jobs chain as regular chain (no hook)", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			// First chain is the jobs chain
			jobsChain := fakeConn.AddChainArgsForCall(0)
			Expect(jobsChain.Name).To(Equal("monit_access_jobs"))
			Expect(jobsChain.Type).To(Equal(nftables.ChainType(""))) // Regular chain has no type
			Expect(jobsChain.Hooknum).To(BeNil())                    // Regular chain has no hook
			Expect(jobsChain.Priority).To(BeNil())                   // Regular chain has no priority
		})

		It("creates monit chain with correct configuration", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			// Second chain is the monit chain (base chain with hook)
			monitChain := fakeConn.AddChainArgsForCall(1)
			Expect(monitChain.Name).To(Equal("monit_access"))
			Expect(monitChain.Type).To(Equal(nftables.ChainTypeFilter))
			Expect(monitChain.Hooknum).NotTo(BeNil())
			Expect(*monitChain.Hooknum).To(Equal(*nftables.ChainHookOutput))
		})

		It("adds jump to jobs chain as first rule", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			// First rule should be the jump rule
			jumpRule := fakeConn.AddRuleArgsForCall(0)
			Expect(jumpRule.Chain.Name).To(Equal("monit_access"))
			Expect(jumpRule.Exprs).To(HaveLen(1))

			verdict, ok := jumpRule.Exprs[0].(*expr.Verdict)
			Expect(ok).To(BeTrue())
			Expect(verdict.Kind).To(Equal(expr.VerdictJump))
			Expect(verdict.Chain).To(Equal("monit_access_jobs"))
		})

		It("adds allow rule for UID 0 after jump rule", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			// Second rule should be the allow rule (has UID match expressions)
			allowRule := fakeConn.AddRuleArgsForCall(1)
			Expect(allowRule.Chain.Name).To(Equal("monit_access"))
			// The allow rule has more expressions (UID match + loopback + port + accept)
			// Block rule has fewer (loopback + port + drop)
			blockRule := fakeConn.AddRuleArgsForCall(2)
			Expect(len(allowRule.Exprs)).To(BeNumerically(">", len(blockRule.Exprs)))
		})

		It("flushes monit chain before adding rules", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConn.FlushChainCallCount()).To(Equal(1))
			flushedChain := fakeConn.FlushChainArgsForCall(0)
			Expect(flushedChain.Name).To(Equal("monit_access"))
		})

		It("never flushes jobs chain to preserve job-managed rules", func() {
			// Call SetupMonitFirewall multiple times to simulate agent restarts
			for i := 0; i < 3; i++ {
				err := manager.SetupMonitFirewall()
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify that all FlushChain calls were on monit_access, never on monit_access_jobs
			flushCount := fakeConn.FlushChainCallCount()
			Expect(flushCount).To(Equal(3)) // Once per call

			for i := 0; i < flushCount; i++ {
				flushedChain := fakeConn.FlushChainArgsForCall(i)
				Expect(flushedChain.Name).To(Equal("monit_access"),
					"FlushChain should only be called on monit_access, not monit_access_jobs")
			}
		})

		Context("when called multiple times", func() {
			It("flushes monit chain each time to prevent duplicate rules", func() {
				err := manager.SetupMonitFirewall()
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeConn.FlushChainCallCount()).To(Equal(1))

				err = manager.SetupMonitFirewall()
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeConn.FlushChainCallCount()).To(Equal(2))

				// Both flush calls should be on the monit chain, not the jobs chain
				Expect(fakeConn.FlushChainArgsForCall(0).Name).To(Equal("monit_access"))
				Expect(fakeConn.FlushChainArgsForCall(1).Name).To(Equal("monit_access"))
			})
		})

		Context("when Flush fails", func() {
			It("returns an error", func() {
				fakeConn.FlushReturns(errors.New("flush failed"))

				err := manager.SetupMonitFirewall()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Flushing nftables rules"))
			})
		})
	})

	Describe("EnableMonitAccess", func() {
		var boshTable *nftables.Table

		BeforeEach(func() {
			boshTable = &nftables.Table{
				Name:   "bosh_agent",
				Family: nftables.TableFamilyINet,
			}
		})

		Context("when the jobs chain exists", func() {
			BeforeEach(func() {
				fakeConn.ListTablesReturns([]*nftables.Table{boshTable}, nil)
				fakeConn.ListChainsReturns([]*nftables.Chain{
					{
						Name:  "monit_access_jobs",
						Table: boshTable,
					},
				}, nil)
			})

			It("adds a rule and flushes", func() {
				err := manager.EnableMonitAccess()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				Expect(fakeConn.FlushCallCount()).To(Equal(1))
			})

			It("adds the rule to the monit_access_jobs chain", func() {
				err := manager.EnableMonitAccess()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(1))
				rule := fakeConn.AddRuleArgsForCall(0)
				fmt.Printf("rule: %+v\n", rule)
				Expect(rule.Chain.Name).To(Equal("monit_access_jobs"))
			})

			It("adds a rule targeting loopback and monit port", func() {
				err := manager.EnableMonitAccess()
				Expect(err).NotTo(HaveOccurred())

				rule := fakeConn.AddRuleArgsForCall(0)
				hasAcceptVerdict := false
				for _, e := range rule.Exprs {
					if verdict, ok := e.(*expr.Verdict); ok {
						if verdict.Kind == expr.VerdictAccept {
							hasAcceptVerdict = true
						}
					}
				}
				Expect(hasAcceptVerdict).To(BeTrue(), "rule should have an accept verdict")
			})

			Context("when the rule already exists (idempotency)", func() {
				It("does not add a duplicate UID rule", func() {
					// Simulate an existing UID rule matching the current UID
					uid := uint32(os.Getuid())
					uidBytes := make([]byte, 4)
					binary.NativeEndian.PutUint32(uidBytes, uid)

					existingRule := &nftables.Rule{
						Exprs: []expr.Any{
							&expr.Meta{Key: expr.MetaKeySKUID, Register: 1},
							&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: uidBytes},
						},
					}

					// First call to GetRules is from cleanupStaleJobRules (cgroup path),
					// subsequent calls check for existing rules.
					// The UID fallback path calls GetRules to check for duplicates.
					fakeConn.GetRulesReturns([]*nftables.Rule{existingRule}, nil)

					err := manager.EnableMonitAccess()
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeConn.AddRuleCallCount()).To(Equal(0))
					Expect(fakeConn.FlushCallCount()).To(Equal(0))
				})
			})

			Context("when Flush fails", func() {
				It("returns an error", func() {
					fakeConn.FlushReturns(errors.New("flush failed"))

					err := manager.EnableMonitAccess()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("flush"))
				})
			})
		})

		Context("when listing tables fails", func() {
			BeforeEach(func() {
				fakeConn.ListTablesReturns(nil, errors.New("listing tables failed"))
			})

			It("returns an error", func() {
				err := manager.EnableMonitAccess()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to check if jobs chain exists"))
			})
		})

		Context("when the bosh_agent table does not exist", func() {
			BeforeEach(func() {
				fakeConn.ListTablesReturns([]*nftables.Table{
					{Name: "some_other_table", Family: nftables.TableFamilyINet},
				}, nil)
			})

			It("returns bosh_agent table not found error", func() {
				err := manager.EnableMonitAccess()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bosh_agent table not found"))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(0))
			})
		})

		Context("when listing chains fails", func() {
			BeforeEach(func() {
				fakeConn.ListTablesReturns([]*nftables.Table{boshTable}, nil)
				fakeConn.ListChainsReturns(nil, errors.New("listing chains failed"))
			})

			It("returns an error", func() {
				err := manager.EnableMonitAccess()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to check if jobs chain exists"))
				Expect(err.Error()).To(ContainSubstring("listing chains"))
			})
		})

		Context("when the table does not have the monit_access_jobs chain", func() {
			BeforeEach(func() {
				fakeConn.ListTablesReturns([]*nftables.Table{boshTable}, nil)
				fakeConn.ListChainsReturns([]*nftables.Chain{}, nil)
			})

			It("returns monit_access_jobs chain not found error", func() {
				err := manager.EnableMonitAccess()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("monit_access_jobs chain not found"))
			})
		})
	})

	Describe("SetupNATSFirewall", func() {
		Context("with an IPv4 address URL", func() {
			It("creates rules for the IPv4 address", func() {
				err := manager.SetupNATSFirewall("nats://user:pass@192.168.1.100:4222")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddTableCallCount()).To(Equal(1))
				Expect(fakeConn.AddChainCallCount()).To(Equal(1))
				Expect(fakeConn.FlushChainCallCount()).To(Equal(1))
				// One allow rule + one block rule
				Expect(fakeConn.AddRuleCallCount()).To(Equal(2))
				Expect(fakeConn.FlushCallCount()).To(Equal(1))
			})

			It("creates chain with correct configuration", func() {
				err := manager.SetupNATSFirewall("nats://192.168.1.100:4222")
				Expect(err).NotTo(HaveOccurred())

				chain := fakeConn.AddChainArgsForCall(0)
				Expect(chain.Name).To(Equal("nats_access"))
				Expect(chain.Type).To(Equal(nftables.ChainTypeFilter))
				Expect(chain.Hooknum).To(Equal(nftables.ChainHookOutput))
			})
		})

		Context("with an IPv6 address URL", func() {
			It("creates rules for the IPv6 address", func() {
				err := manager.SetupNATSFirewall("nats://user:pass@[2001:db8::1]:4222")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(2))
				Expect(fakeConn.FlushCallCount()).To(Equal(1))
			})
		})

		Context("with a hostname URL", func() {
			It("resolves DNS and creates rules for resolved IPs", func() {
				fakeResolver.LookupIPReturns([]net.IP{
					net.ParseIP("10.0.0.1"),
					net.ParseIP("10.0.0.2"),
				}, nil)

				err := manager.SetupNATSFirewall("nats://user:pass@nats.example.com:4222")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeResolver.LookupIPCallCount()).To(Equal(1))
				Expect(fakeResolver.LookupIPArgsForCall(0)).To(Equal("nats.example.com"))

				// Two IPs * 2 rules each = 4 rules
				Expect(fakeConn.AddRuleCallCount()).To(Equal(4))
			})

			It("handles DNS resolution failure gracefully", func() {
				fakeResolver.LookupIPReturns(nil, errors.New("dns lookup failed"))

				err := manager.SetupNATSFirewall("nats://user:pass@nats.example.com:4222")
				Expect(err).NotTo(HaveOccurred()) // Should not return error, just log warning

				Expect(fakeResolver.LookupIPCallCount()).To(Equal(1))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(0)) // No rules added
			})
		})

		Context("with default port", func() {
			It("uses port 4222 when not specified", func() {
				err := manager.SetupNATSFirewall("nats://192.168.1.100")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(2))
			})
		})

		Context("with custom port", func() {
			It("uses the specified port", func() {
				err := manager.SetupNATSFirewall("nats://192.168.1.100:5222")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddRuleCallCount()).To(Equal(2))
			})
		})

		Context("with https URL", func() {
			It("skips setup and returns nil", func() {
				err := manager.SetupNATSFirewall("https://director.example.com:25555")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddTableCallCount()).To(Equal(0))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(0))
			})
		})

		Context("with empty URL", func() {
			It("skips setup and returns nil", func() {
				err := manager.SetupNATSFirewall("")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeConn.AddTableCallCount()).To(Equal(0))
				Expect(fakeConn.AddRuleCallCount()).To(Equal(0))
			})
		})

		Context("when called multiple times", func() {
			It("flushes existing NATS chain before adding new rules", func() {
				err := manager.SetupNATSFirewall("nats://192.168.1.100:4222")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeConn.FlushChainCallCount()).To(Equal(1))

				err = manager.SetupNATSFirewall("nats://192.168.1.200:4222")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeConn.FlushChainCallCount()).To(Equal(2))
			})
		})

		Context("when Flush fails", func() {
			It("returns an error", func() {
				fakeConn.FlushReturns(errors.New("flush failed"))

				err := manager.SetupNATSFirewall("nats://192.168.1.100:4222")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Flushing nftables rules"))
			})
		})
	})

	Describe("BeforeConnect", func() {
		var hook firewall.NatsFirewallHook

		BeforeEach(func() {
			hook = manager.(firewall.NatsFirewallHook)
		})

		It("delegates to SetupNATSFirewall", func() {
			err := hook.BeforeConnect("nats://192.168.1.100:4222")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConn.AddTableCallCount()).To(Equal(1))
			Expect(fakeConn.AddChainCallCount()).To(Equal(1))
			Expect(fakeConn.AddRuleCallCount()).To(Equal(2))
		})

		It("returns nil on success", func() {
			err := hook.BeforeConnect("nats://192.168.1.100:4222")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns error when SetupNATSFirewall fails", func() {
			fakeConn.FlushReturns(errors.New("flush failed"))

			err := hook.BeforeConnect("nats://192.168.1.100:4222")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Manager interface implementation", func() {
		It("implements NatsFirewallHook interface", func() {
			hook := manager.(firewall.NatsFirewallHook)
			Expect(hook).NotTo(BeNil())
		})
	})
})
