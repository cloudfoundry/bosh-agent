package agentlogger_test

import (
	"bytes"
	"os"
	"syscall"

	"github.com/cloudfoundry/bosh-agent/v2/infrastructure/agentlogger"
	"github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Signal-able logger debug", func() {
	Describe("when SIGSEGV is received", func() {
		It("it dumps all goroutines to the given buffer", func() {
			outBuf := new(bytes.Buffer)
			signalChannel := make(chan os.Signal, 1)
			writerLogger := logger.NewWriterLogger(logger.LevelError, outBuf)
			_, doneChannel := agentlogger.NewSignalableLogger(writerLogger, signalChannel)

			signalChannel <- syscall.SIGSEGV
			<-doneChannel

			Expect(outBuf).To(ContainSubstring("Dumping goroutines"))
			Expect(outBuf).To(MatchRegexp(`goroutine (\d+) \[(syscall|running)\]`))
		})
	})
})
