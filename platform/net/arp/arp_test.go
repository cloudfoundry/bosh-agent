package arp_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net/arp"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func expectedLogFormat(level, msg string) string {
	return fmt.Sprintf("\\[arp] [0-9]{4}/[0-9]{2}/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} %s - %s\n", level, msg)
}

var _ = Describe("arp", func() {

	var (
		address   string
		arp       Arp
		cmdRunner *fakesys.FakeCmdRunner
		logger    boshlog.Logger
		outBuf    *bytes.Buffer
		errBuf    *bytes.Buffer
	)

	BeforeEach(func() {
		address = "10.0.0.1"
		cmdRunner = fakesys.NewFakeCmdRunner()
		outBuf = bytes.NewBufferString("")
		errBuf = bytes.NewBufferString("")
		logger = boshlog.NewWriterLogger(boshlog.LevelDebug, outBuf, errBuf)
		arp = NewArp(cmdRunner, logger)
	})

	Describe("Delete", func() {
		JustBeforeEach(func() {
			arp.Delete(address)
		})

		It("Logs as DEBUG before attempting arp deletion", func() {
			expectedContent := expectedLogFormat("DEBUG", fmt.Sprintf("Deleting.*%s.*", address))
			Expect(outBuf).To(MatchRegexp(expectedContent))
		})

		It("runs arp -d for the given IP", func() {
			expectedCommand := []string{"arp", "-d", address}
			cmd := cmdRunner.RunCommands[0]
			Expect(reflect.DeepEqual(cmd, expectedCommand)).To(BeTrue())
		})

		Context("When ARP deletion command returns an error", func() {
			BeforeEach(func() {
				cmd := fmt.Sprintf("arp -d %s", address)
				errorResult := fakesys.FakeCmdResult{
					Stdout:     "",
					Stderr:     "",
					ExitStatus: 1,
					Error:      errors.New("an error"),
					Sticky:     false,
				}
				cmdRunner.AddCmdResult(cmd, errorResult)
			})

			It("logs the error as INFO", func() {
				expectedContent := expectedLogFormat("INFO", fmt.Sprintf("Ignoring arp failure deleting %s from cache:.*", address))
				Expect(outBuf).To(MatchRegexp(expectedContent))
			})
		})
	})

})
