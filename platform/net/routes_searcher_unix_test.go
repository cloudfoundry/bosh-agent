//go:build !windows
// +build !windows

package net_test

import (
	"errors"

	"github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/net"
)

var _ = Describe("cmdRoutesSeacher", func() {
	var (
		runner   *fakesys.FakeCmdRunner
		searcher RoutesSearcher
	)

	BeforeEach(func() {
		runner = fakesys.NewFakeCmdRunner()
		searcher = NewRoutesSearcher(&loggerfakes.FakeLogger{}, runner, nil)
	})

	Describe("SearchRoutes", func() {
		Context("when running command succeeds", func() {
			It("returns parsed routes information", func() {
				runner.AddCmdResult("ip r", fakesys.FakeCmdResult{
					Stdout: `172.16.79.0/24 dev eth0 proto kernel scope link metric 100
169.254.0.0/16 dev eth0 proto link metric 1000
default via 172.16.79.1 dev eth0 proto dhcp metric 100
`,
				})

				routes, err := searcher.SearchRoutes(iptables.ProtocolIPv4)
				Expect(err).ToNot(HaveOccurred())
				Expect(runner.RunCommandsQuietly[0]).To(Equal([]string{"ip", "r"}))
				Expect(routes).To(Equal([]Route{
					Route{Destination: "172.16.79.0", Gateway: "0.0.0.0", InterfaceName: "eth0"},
					Route{Destination: "169.254.0.0", Gateway: "0.0.0.0", InterfaceName: "eth0"},
					Route{Destination: "0.0.0.0", Gateway: "172.16.79.1", InterfaceName: "eth0"},
				}))
			})

			It("ignores unexpected blackhole routes information", func() {
				runner.AddCmdResult("ip r", fakesys.FakeCmdResult{
					Stdout: `172.16.79.0/24 dev eth0 proto kernel scope link metric 100
169.254.0.0/16 dev eth0 proto link metric 1000
default via 172.16.79.1 dev eth0 proto dhcp metric 100
blackhole 10.200.115.192/26  proto bird
`,
				})

				routes, err := searcher.SearchRoutes(iptables.ProtocolIPv4)
				Expect(err).ToNot(HaveOccurred())
				Expect(runner.RunCommandsQuietly[0]).To(Equal([]string{"ip", "r"}))
				Expect(routes).To(Equal([]Route{
					Route{Destination: "172.16.79.0", Gateway: "0.0.0.0", InterfaceName: "eth0"},
					Route{Destination: "169.254.0.0", Gateway: "0.0.0.0", InterfaceName: "eth0"},
					Route{Destination: "0.0.0.0", Gateway: "172.16.79.1", InterfaceName: "eth0"},
				}))
			})

			It("ignores empty lines for ipv4", func() {
				runner.AddCmdResult("ip r", fakesys.FakeCmdResult{
					Stdout: `
					
`,
				})

				routes, err := searcher.SearchRoutes(iptables.ProtocolIPv4)
				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(BeEmpty())
			})

			It("ignores empty lines for ipv6", func() {
				runner.AddCmdResult("ip -6 r", fakesys.FakeCmdResult{
					Stdout: `

`,
				})

				routes, err := searcher.SearchRoutes(iptables.ProtocolIPv6)
				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(BeEmpty())
			})

			It("returns parsed routes information for ipv6", func() {
				runner.AddCmdResult("ip -6 r", fakesys.FakeCmdResult{
					Stdout: `
::1 dev lo proto kernel metric 256 pref medium
2600:1f18:58fb:2009::/64 dev eth0 proto ra metric 1024 pref medium
fe80::/64 dev eth0 proto kernel metric 256 pref medium
default via fe80::ceb:d3ff:fef9:fa93 dev eth0 proto ra metric 1024 expires 1796sec pref medium
`,
				})

				routes, err := searcher.SearchRoutes(iptables.ProtocolIPv6)
				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(Equal([]Route{
					Route{Destination: "::1", Gateway: "::", InterfaceName: "lo"},
					Route{Destination: "2600:1f18:58fb:2009::", Gateway: "::", InterfaceName: "eth0"},
					Route{Destination: "fe80::", Gateway: "::", InterfaceName: "eth0"},
					Route{Destination: "::", Gateway: "fe80::ceb:d3ff:fef9:fa93", InterfaceName: "eth0"},
				}))
			})

		})

		Context("when running ip command fails", func() {
			It("returns error", func() {
				runner.AddCmdResult("ip r", fakesys.FakeCmdResult{
					Error: errors.New("fake-run-err"),
				})

				routes, err := searcher.SearchRoutes(iptables.ProtocolIPv4)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-err"))
				Expect(routes).To(BeEmpty())
			})
		})
	})
})
