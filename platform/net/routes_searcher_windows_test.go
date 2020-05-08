package net_test

import (
	"github.com/cloudfoundry/bosh-agent/platform/net"
	fakenet "github.com/cloudfoundry/bosh-agent/platform/net/fakes"
	fakelogger "github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/onsi/ginkgo"
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

			routes, err := searcher.SearchRoutes()
			Expect(err).NotTo(HaveOccurred())
			Expect(routes).To(HaveLen(2))

			Expect(routes[0].InterfaceName).To(Equal("some-created-interface"))
			Expect(routes[0].Gateway).To(Equal("172.30.0.1"))
			Expect(routes[0].IsDefault()).To(BeFalse())

			Expect(routes[1].InterfaceName).To(Equal("some-default-interface"))
			Expect(routes[1].Gateway).To(Equal("10.0.16.1"))
			Expect(routes[1].IsDefault()).To(BeTrue())
		})
	})
})
