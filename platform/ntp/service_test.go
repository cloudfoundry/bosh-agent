package ntp_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/ntp"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("concreteService", func() {
	Describe("GetInfo", func() {
		var (
			cmdRunner *fakesys.FakeCmdRunner
			cmd       string
			service   Service
		)

		BeforeEach(func() {
			cmdRunner = fakesys.NewFakeCmdRunner()
			service = NewConcreteService(cmdRunner)
			cmd = "sh -c ntpq -c 'readvar 0 clock,offset'"
		})

		It("returns valid offset", func() {
			NTPData := "offset=1.299, clock=da015395.2ef3fae1  Thu, Nov 26 2015 17:47:01.183"
			cmdRunner.AddCmdResult(cmd, fakesys.FakeCmdResult{Stdout: NTPData})

			expectedNTPOffset := Info{
				Timestamp: "26 Nov 17:47:01",
				Offset:    "1.299",
			}
			Expect(service.GetInfo()).To(Equal(expectedNTPOffset))
		})

		It("returns ntp is not started when ntp connection timed out", func() {
			NTPData := "timed out"
			cmdRunner.AddCmdResult(cmd, fakesys.FakeCmdResult{Stdout: NTPData})

			expectedNTPOffset := Info{
				Message: "ntp service is not available",
			}
			Expect(service.GetInfo()).To(Equal(expectedNTPOffset))
		})

		It("returns error querying time by ntpq when bad response from ntpd is received", func() {
			NTPData := "abcdefg\n"
			cmdRunner.AddCmdResult(cmd, fakesys.FakeCmdResult{Stdout: NTPData})

			expectedNTPOffset := Info{
				Message: "error querying time by ntpq",
			}
			Expect(service.GetInfo()).To(Equal(expectedNTPOffset))
		})
	})
})
