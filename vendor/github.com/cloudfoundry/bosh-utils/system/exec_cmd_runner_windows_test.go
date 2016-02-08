// +build windows

package system_test

import (
	"os"
	"strings"

	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-utils/system"
)

var _ = Describe("execCmdRunner", func() {
	var (
		runner CmdRunner
	)

	BeforeEach(func() {
		runner = NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelNone))
	})

	Describe("RunComplexCommandAsync", func() {
		It("runs and exits", func() {
			cmd := Command{Name: "cmd.exe", Args: []string{"/C", "dir"}}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := Result{}
			Eventually(process.Wait()).Should(Receive(&result))
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(0))
		})

		It("populates stdout and stderr", func() {
			cmd := Command{Name: "cmd.exe", Args: []string{"/C", "echo stdout & echo stderr 1>&2"}}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := Result{}
			Eventually(process.Wait()).Should(Receive(&result))
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(strings.TrimSpace(result.Stdout)).To(Equal("stdout"))
			Expect(strings.TrimSpace(result.Stderr)).To(Equal("stderr"))
		})

		It("returns error and sets status to exit status of comamnd if it command exits with non-0 status", func() {
			cmd := Command{Name: "cmd.exe", Args: []string{"/C", "exit /b 10"}}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := Result{}
			Eventually(process.Wait()).Should(Receive(&result))
			Expect(result.Error).To(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(10))
		})

		It("allows setting custom env variable in addition to inheriting process env variables", func() {
			cmd := Command{
				Name: "powershell.exe",
				Args: []string{"Get-ChildItem", "Env:"},
				Env: map[string]string{
					"FOO": "BAR",
				},
			}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := Result{}
			Eventually(process.Wait()).Should(Receive(&result))
			Expect(result.Error).ToNot(HaveOccurred())
			stdout := strings.Split(result.Stdout, "\n")
			ok := false
			for _, line := range stdout {
				fields := strings.Fields(line)
				if len(fields) == 2 && fields[0] == "FOO" && fields[1] == "BAR" {
					ok = true
					break
				}
			}
			Expect(ok).To(Equal(true))
		})

		It("changes working dir", func() {
			tmpDir := os.TempDir()
			Expect(tmpDir).ToNot(Equal(""))
			cmd := Command{Name: "cmd.exe", Args: []string{"/C", "echo %CD%"}, WorkingDir: tmpDir}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := Result{}
			Eventually(process.Wait()).Should(Receive(&result))
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.Stdout).To(ContainSubstring(tmpDir))
		})
	})
})
