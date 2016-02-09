// +build windows

package system_test

import (
	"os/exec"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-utils/system"
)

var _ = Describe("execCmdRunner", func() {
	Describe("Start", func() {
		var (
			logger boshlog.Logger
		)

		BeforeEach(func() {
			logger = boshlog.NewLogger(boshlog.LevelNone)
		})

		It("runs and exits", func() {
			command := exec.Command("cmd.exe", "/C", "dir")
			process := NewExecProcess(command, logger)
			err := process.Start()
			Expect(err).ToNot(HaveOccurred())

			result := Result{}
			Eventually(process.Wait()).Should(Receive(&result))
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(0))
		})
	})
})
