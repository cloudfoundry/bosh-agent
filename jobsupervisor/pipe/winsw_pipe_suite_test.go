package main

import (
	"runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var pathToPipeCLI string
var GoSequencePath string
var shell string
var echoCmdArgs []string

const echoOutput = "hello"

func TestWinswPipe(t *testing.T) {
	BeforeSuite(func() {
		var err error
		pathToPipeCLI, err = gexec.Build("github.com/cloudfoundry/bosh-agent/jobsupervisor/pipe")
		Expect(err).To(Succeed())

		GoSequencePath, err = gexec.Build("./testdata/gosequence.go")
		Expect(err).To(Succeed())
	})

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			shell = "powershell"
			echoCmdArgs = []string{shell, "-c", "echo", echoOutput}
			SetDefaultEventuallyTimeout(5 * time.Second)
		} else {
			echoCmdArgs = []string{"echo", echoOutput}
			shell = "bash"
		}
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "WinswPipe Suite")
}
