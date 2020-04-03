// +build !windows

package net_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
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

				routes, err := searcher.SearchRoutes()
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

				routes, err := searcher.SearchRoutes()
				Expect(err).ToNot(HaveOccurred())
				Expect(runner.RunCommandsQuietly[0]).To(Equal([]string{"ip", "r"}))
				Expect(routes).To(Equal([]Route{
					Route{Destination: "172.16.79.0", Gateway: "0.0.0.0", InterfaceName: "eth0"},
					Route{Destination: "169.254.0.0", Gateway: "0.0.0.0", InterfaceName: "eth0"},
					Route{Destination: "0.0.0.0", Gateway: "172.16.79.1", InterfaceName: "eth0"},
				}))
			})

			It("ignores empty lines", func() {
				runner.AddCmdResult("ip r", fakesys.FakeCmdResult{
					Stdout: `
`,
				})

				routes, err := searcher.SearchRoutes()
				Expect(err).ToNot(HaveOccurred())
				Expect(routes).To(BeEmpty())
			})
		})

		Context("when running ip command fails", func() {
			It("returns error", func() {
				runner.AddCmdResult("ip r", fakesys.FakeCmdResult{
					Error: errors.New("fake-run-err"),
				})

				routes, err := searcher.SearchRoutes()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-err"))
				Expect(routes).To(BeEmpty())
			})
		})
	})
})
