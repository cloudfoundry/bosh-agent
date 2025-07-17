package net_test

import (
	fakelogger "github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	"github.com/cloudfoundry/bosh-agent/v2/platform/net"
	fakenet "github.com/cloudfoundry/bosh-agent/v2/platform/net/fakes"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Windows Route Searcher", func() {
	var (
		runner           *fakesys.FakeCmdRunner
		interfaceManager *fakenet.FakeInterfaceManager
		logger           *fakelogger.FakeLogger
		searcher         net.RoutesSearcher
	)

	BeforeEach(func() {
		runner = fakesys.NewFakeCmdRunner()
		interfaceManager = &fakenet.FakeInterfaceManager{}
		searcher = net.NewRoutesSearcher(logger, runner, interfaceManager)
	})

	Describe("SeachRoutes", func() {
		It("returns default and non-default routes for existing interfaces", func() {
			interfaceManager.GetInterfacesInterfaces = []net.Interface{
				{Name: "some-created-interface", Gateway: "172.30.0.1"},
				{Name: "some-default-interface", Gateway: "10.0.16.1"},
			}
			runner.AddCmdResult("(Get-NetRoute -DestinationPrefix '0.0.0.0/0').NextHop", fakesys.FakeCmdResult{
				Stdout: `10.0.16.1`,
			})

			routes, err := searcher.SearchRoutes(boship.IPv4)
			Expect(err).NotTo(HaveOccurred())
			Expect(routes).To(HaveLen(2))

			Expect(routes[0].InterfaceName).To(Equal("some-created-interface"))
			Expect(routes[0].Gateway).To(Equal("172.30.0.1"))
			Expect(routes[0].IsDefault(boship.IPv4)).To(BeFalse())

			Expect(routes[1].InterfaceName).To(Equal("some-default-interface"))
			Expect(routes[1].Gateway).To(Equal("10.0.16.1"))
			Expect(routes[1].IsDefault(boship.IPv4)).To(BeTrue())
		})

		It("returns default and non-default routes for existing interfaces", func() {
			interfaceManager.GetInterfacesInterfaces = []net.Interface{
				{Name: "some-created-interface", Gateway: "2600:1000::1"},
				{Name: "some-default-interface", Gateway: "10.0.16.1"},
			}
			runner.AddCmdResult("(Get-NetRoute -DestinationPrefix '::/0').NextHop", fakesys.FakeCmdResult{
				Stdout: `2600:1000::1`,
			})

			routes, err := searcher.SearchRoutes(boship.IPv6)
			Expect(err).NotTo(HaveOccurred())
			Expect(routes).To(HaveLen(2))

			Expect(routes[0].InterfaceName).To(Equal("some-created-interface"))
			Expect(routes[0].Gateway).To(Equal("2600:1000::1"))
			Expect(routes[0].IsDefault(boship.IPv6)).To(BeTrue())

			Expect(routes[1].InterfaceName).To(Equal("some-default-interface"))
			Expect(routes[1].Gateway).To(Equal("10.0.16.1"))
			Expect(routes[1].IsDefault(boship.IPv6)).To(BeFalse())
		})
	})
})
