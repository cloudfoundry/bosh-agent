//go:build linux

package firewall_test

import (
	"errors"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/google/nftables"

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
		It("creates table, chain, and rules successfully", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConn.AddTableCallCount()).To(Equal(1))
			Expect(fakeConn.AddChainCallCount()).To(Equal(1))
			Expect(fakeConn.AddRuleCallCount()).To(Equal(2)) // allow + block
			Expect(fakeConn.FlushCallCount()).To(Equal(1))
		})

		It("creates table with correct configuration", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			table := fakeConn.AddTableArgsForCall(0)
			Expect(table.Name).To(Equal("bosh_agent"))
			Expect(table.Family).To(Equal(nftables.TableFamilyINet))
		})

		It("creates chain with correct configuration", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			chain := fakeConn.AddChainArgsForCall(0)
			Expect(chain.Name).To(Equal("monit_access"))
			Expect(chain.Type).To(Equal(nftables.ChainTypeFilter))
			Expect(chain.Hooknum).To(Equal(nftables.ChainHookOutput))
		})

		It("creates allow rule for UID 0 first", func() {
			err := manager.SetupMonitFirewall()
			Expect(err).NotTo(HaveOccurred())

			// First rule should be the allow rule (has more expressions for UID match)
			firstRule := fakeConn.AddRuleArgsForCall(0)
			Expect(firstRule.Exprs).NotTo(BeEmpty())
			// The allow rule has more expressions (UID match + loopback + port + accept)
			// Block rule has fewer (loopback + port + drop)
			secondRule := fakeConn.AddRuleArgsForCall(1)
			Expect(len(firstRule.Exprs)).To(BeNumerically(">", len(secondRule.Exprs)))
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
