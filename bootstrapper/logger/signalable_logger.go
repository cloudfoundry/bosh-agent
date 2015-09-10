package logger

import (
	"github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	"os"
	"runtime"
)

func NewSignalableLogger(writerLogger logger.Logger, signalChannel chan os.Signal) (logger.Logger, chan bool) {
	stackTrace := make([]byte, 10000)
	doneChannel := make(chan bool, 1)
	go func() {
		for {
			<-signalChannel
			writerLogger.Error("Received SIGSEGV", "Dumping goroutines...")
			runtime.Stack(stackTrace, true)
			writerLogger.Error("Received SIGSEGV", string(stackTrace))
			doneChannel <- true
		}
	}()
	return writerLogger, doneChannel
}
