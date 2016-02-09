package system_test

import (
	"os"
	"runtime"
	"strings"

	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("execCmdRunner", func() {
	var (
		runner CmdRunner
	)

	BeforeEach(func() {
		runner = NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelNone))
	})

	Describe("RunComplexCommand", func() {
		It("run complex command with working directory", func() {
			cmd := Command{
				Name:       "ls",
				Args:       []string{"-l"},
				WorkingDir: ".",
			}
			stdout, stderr, status, err := runner.RunComplexCommand(cmd)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("exec_cmd_runner_fixtures"))
			Expect(stdout).To(ContainSubstring("total"))
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))
		})

		It("run complex command with env", func() {
			cmd := Command{
				Name: "env",
				Env: map[string]string{
					"FOO": "BAR",
				},
			}
			stdout, stderr, status, err := runner.RunComplexCommand(cmd)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("FOO=BAR"))
			Expect(stdout).To(ContainSubstring("PATH="))
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))
		})

		It("runs complex command with specific env", func() {
			cmd := Command{
				Name: "env",
				Env: map[string]string{
					"FOO": "BAR",
				},
				UseIsolatedEnv: true,
			}
			stdout, stderr, status, err := runner.RunComplexCommand(cmd)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("FOO=BAR"))
			Expect(stdout).ToNot(ContainSubstring("PATH="))
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))
		})

		It("run complex command with stdin", func() {
			input := "This is STDIN\nWith another line."
			cmd := Command{
				Name:  "cat",
				Args:  []string{"/dev/stdin"},
				Stdin: strings.NewReader(input),
			}
			stdout, stderr, status, err := runner.RunComplexCommand(cmd)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Equal(input))
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))
		})

		It("prints stdout/stderr to provided I/O object", func() {
			fs := fakesys.NewFakeFileSystem()
			stdoutFile, err := fs.OpenFile("/fake-stdout-path", os.O_RDWR, os.FileMode(0644))
			Expect(err).ToNot(HaveOccurred())

			stderrFile, err := fs.OpenFile("/fake-stderr-path", os.O_RDWR, os.FileMode(0644))
			Expect(err).ToNot(HaveOccurred())

			cmd := Command{
				Name:   "bash",
				Args:   []string{"-c", "echo fake-out >&1; echo fake-err >&2"},
				Stdout: stdoutFile,
				Stderr: stderrFile,
			}

			stdout, stderr, status, err := runner.RunComplexCommand(cmd)
			Expect(err).ToNot(HaveOccurred())

			Expect(stdout).To(BeEmpty())
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))

			stdoutContents := make([]byte, 1024)
			_, err = stdoutFile.Read(stdoutContents)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(stdoutContents)).To(ContainSubstring("fake-out"))

			stderrContents := make([]byte, 1024)
			_, err = stderrFile.Read(stderrContents)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(stderrContents)).To(ContainSubstring("fake-err"))
		})
	})

	Describe("RunComplexCommandAsync", func() {
		It("populates stdout and stderr", func() {
			cmd := Command{Name: "ls"}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := <-process.Wait()
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(0))
		})

		It("populates stdout and stderr", func() {
			cmd := Command{Name: "bash", Args: []string{"-c", "echo stdout >&1; echo stderr >&2"}}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := <-process.Wait()
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.Stdout).To(Equal("stdout\n"))
			Expect(result.Stderr).To(Equal("stderr\n"))
		})

		It("returns error and sets status to exit status of comamnd if it command exits with non-0 status", func() {
			cmd := Command{Name: "bash", Args: []string{"-c", "exit 10"}}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := <-process.Wait()
			Expect(result.Error).To(HaveOccurred())
			Expect(result.ExitStatus).To(Equal(10))
		})

		It("allows setting custom env variable in addition to inheriting process env variables", func() {
			cmd := Command{
				Name: "env",
				Env: map[string]string{
					"FOO": "BAR",
				},
			}
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := <-process.Wait()
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.Stdout).To(ContainSubstring("FOO=BAR"))
			Expect(result.Stdout).To(ContainSubstring("PATH="))
		})

		It("changes working dir", func() {
			cmd := GetPlatformCommand("pwd")
			process, err := runner.RunComplexCommandAsync(cmd)
			Expect(err).ToNot(HaveOccurred())

			result := <-process.Wait()
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(result.Stdout).To(ContainSubstring(cmd.WorkingDir))
		})
	})

	Describe("RunCommand", func() {
		It("run command", func() {
			stdout, stderr, status, err := runner.RunCommand("echo", "Hello World!")
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Equal("Hello World!\n"))
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))
		})

		It("run command with error output", func() {
			cmd := GetPlatformCommand("stderr")
			stdout, stderr, status, err := runner.RunCommand(cmd.Name, cmd.Args...)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(BeEmpty())
			Expect(stderr).To(ContainSubstring("error-output"))
			Expect(status).To(Equal(0))
		})

		It("run command with non-0 exit status", func() {
			cmd := GetPlatformCommand("exit")
			stdout, stderr, status, err := runner.RunCommand(cmd.Name, cmd.Args...)
			Expect(err).To(HaveOccurred())
			Expect(stdout).To(BeEmpty())
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(14))
		})

		It("run command with error", func() {
			stdout, stderr, status, err := runner.RunCommand("false")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Running command: 'false', stdout: '', stderr: '': exit status 1"))
			Expect(stderr).To(BeEmpty())
			Expect(stdout).To(BeEmpty())
			Expect(status).To(Equal(1))
		})

		It("run command with error with args", func() {
			stdout, stderr, status, err := runner.RunCommand("false", "second arg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Running command: 'false second arg', stdout: '', stderr: '': exit status 1"))
			Expect(stderr).To(BeEmpty())
			Expect(stdout).To(BeEmpty())
			Expect(status).To(Equal(1))
		})

		It("run command with cmd not found", func() {
			stdout, stderr, status, err := runner.RunCommand("something that does not exist")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(stderr).To(BeEmpty())
			Expect(stdout).To(BeEmpty())
			Expect(status).To(Equal(-1))
		})
	})

	Describe("CommandExists", func() {
		It("run command with input", func() {
			stdout, stderr, status, err := runner.RunCommandWithInput("foo\nbar\nbaz", "grep", "ba")
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Equal("bar\nbaz\n"))
			Expect(stderr).To(BeEmpty())
			Expect(status).To(Equal(0))
		})
	})

	Describe("CommandExists", func() {
		It("command exists", func() {
			Expect(runner.CommandExists("env")).To(BeTrue())
			Expect(runner.CommandExists("absolutely-does-not-exist-ever-please-unicorns")).To(BeFalse())
		})
	})
})

func GetPlatformCommand(cmdName string) Command {
	windowsCommands := map[string]Command{
		"pwd": Command{
			Name:       "powershell",
			Args:       []string{"echo $PWD"},
			WorkingDir: `C:\windows\temp`,
		},
		"stderr": Command{
			Name: "powershell",
			Args: []string{"[Console]::Error.WriteLine('error-output')"},
		},
		"exit": Command{
			Name: "powershell",
			Args: []string{"exit 14"},
		},
	}

	unixCommands := map[string]Command{
		"pwd": Command{
			Name:       "bash",
			Args:       []string{"-c", "echo $PWD"},
			WorkingDir: `/tmp`,
		},
		"stderr": Command{
			Name: "bash",
			Args: []string{"-c", "echo error-output >&2"},
		},
		"exit": Command{
			Name: "bash",
			Args: []string{"-c", "exit 14"},
		},
	}

	if runtime.GOOS == "windows" {
		return windowsCommands[cmdName]
	}

	return unixCommands[cmdName]
}
