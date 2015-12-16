// +build !windows

package system_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-utils/internal/github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() {
	Describe("execCmdRunner", func() {
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
				cmd := Command{Name: "bash", Args: []string{"-c", "echo $PWD"}, WorkingDir: "/tmp"}
				process, err := runner.RunComplexCommandAsync(cmd)
				Expect(err).ToNot(HaveOccurred())

				result := <-process.Wait()
				Expect(result.Error).ToNot(HaveOccurred())
				Expect(result.Stdout).To(ContainSubstring("/tmp"))
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
				stdout, stderr, status, err := runner.RunCommand("bash", "-c", "echo error-output >&2")
				Expect(err).ToNot(HaveOccurred())
				Expect(stdout).To(BeEmpty())
				Expect(stderr).To(ContainSubstring("error-output"))
				Expect(status).To(Equal(0))
			})

			It("run command with non-0 exit status", func() {
				stdout, stderr, status, err := runner.RunCommand("bash", "-c", "exit 14")
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

	Describe("execProcess", func() {
		var (
			runner CmdRunner
		)

		BeforeEach(func() {
			runner = NewExecCmdRunner(boshlog.NewLogger(boshlog.LevelNone))
		})

		Describe("TerminateNicely", func() {
			var (
				buildDir string
			)

			hasProcessesFromBuildDir := func() (bool, string) {
				// Make sure to show all processes on the system
				output, err := exec.Command("ps", "-A", "-o", "pid,args").Output()
				Expect(err).ToNot(HaveOccurred())

				// Cannot check for PID existence directly because
				// PID could have been recycled by the OS; make sure it's not the same process
				for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
					if strings.Contains(line, buildDir) {
						return true, line
					}
				}

				return false, ""
			}

			expectProcessesToNotExist := func() {
				exists, ps := hasProcessesFromBuildDir()
				Expect(exists).To(BeFalse(), "Expected following process to not exist %s", ps)
			}

			BeforeEach(func() {
				var (
					err error
				)

				buildDir, err = ioutil.TempDir("", "TerminateNicely")
				Expect(err).ToNot(HaveOccurred())

				exesToCompile := []string{
					"exe_exits",
					"child_ignore_term",
					"child_term",
					"parent_ignore_term",
					"parent_term",
				}

				for _, exe := range exesToCompile {
					dst := filepath.Join(buildDir, exe)
					src := filepath.Join("exec_cmd_runner_fixtures", exe+".go")
					err := exec.Command("go", "build", "-o", dst, src).Run()
					Expect(err).ToNot(HaveOccurred())
				}
			})

			AfterEach(func() {
				os.RemoveAll(buildDir)
			})

			Context("when parent and child terminate after receiving SIGTERM", func() {
				It("sends term signal to the whole group and returns with exit status that parent exited", func() {
					cmd := Command{Name: filepath.Join(buildDir, "parent_term")}
					process, err := runner.RunComplexCommandAsync(cmd)
					Expect(err).ToNot(HaveOccurred())

					// Wait for script to start and output pids
					time.Sleep(2 * time.Second)

					waitCh := process.Wait()

					err = process.TerminateNicely(1 * time.Minute)
					Expect(err).ToNot(HaveOccurred())

					result := <-waitCh
					Expect(result.Error).To(HaveOccurred())

					// Parent exit code is returned
					// bash adds 128 to signal status as exit code
					Expect(result.ExitStatus).To(Equal(13))

					// Term signal was sent to all processes in the group
					Expect(result.Stdout).To(ContainSubstring("Parent received SIGTERM"))
					Expect(result.Stdout).To(ContainSubstring("Child received SIGTERM"))

					// All processes are gone
					expectProcessesToNotExist()
				})
			})

			Context("when parent and child do not exit after receiving SIGTERM in small amount of time", func() {
				It("sends kill signal to the whole group and returns with ? exit status", func() {
					cmd := Command{Name: filepath.Join(buildDir, "parent_ignore_term")}
					process, err := runner.RunComplexCommandAsync(cmd)
					Expect(err).ToNot(HaveOccurred())

					// Wait for script to start and output pids
					time.Sleep(2 * time.Second)

					waitCh := process.Wait()

					err = process.TerminateNicely(2 * time.Second)
					Expect(err).ToNot(HaveOccurred())

					result := <-waitCh
					Expect(result.Error).To(HaveOccurred())

					// Parent exit code is returned
					Expect(result.ExitStatus).To(Equal(128 + 9))

					// Term signal was sent to all processes in the group before kill
					Expect(result.Stdout).To(ContainSubstring("Parent received SIGTERM"))
					Expect(result.Stdout).To(ContainSubstring("Child received SIGTERM"))

					// Parent and child are killed
					expectProcessesToNotExist()
				})
			})

			Context("when parent and child already exited before calling TerminateNicely", func() {
				It("returns without an error since all processes are gone", func() {
					cmd := Command{Name: filepath.Join(buildDir, "exe_exits")}
					process, err := runner.RunComplexCommandAsync(cmd)
					Expect(err).ToNot(HaveOccurred())

					// Wait for script to exit
					for i := 0; i < 20; i++ {
						if exists, _ := hasProcessesFromBuildDir(); !exists {
							break
						}
						if i == 19 {
							Fail("Expected process did not exit fast enough")
						}
						time.Sleep(500 * time.Millisecond)
					}

					waitCh := process.Wait()

					err = process.TerminateNicely(2 * time.Second)
					Expect(err).ToNot(HaveOccurred())

					result := <-waitCh
					Expect(result.Error).ToNot(HaveOccurred())
					Expect(result.Stdout).To(Equal(""))
					Expect(result.Stderr).To(Equal(""))
					Expect(result.ExitStatus).To(Equal(0))
				})
			})
		})
	})

	Describe("ExecError", func() {
		Describe("Error", func() {
			It("returns error message with full stdout and full stderr to aid debugging", func() {
				execErr := NewExecError("fake-cmd", "fake-stdout", "fake-stderr")
				expectedMsg := "Running command: 'fake-cmd', stdout: 'fake-stdout', stderr: 'fake-stderr'"
				Expect(execErr.Error()).To(Equal(expectedMsg))
			})
		})

		Describe("ShortError", func() {
			buildLines := func(start, stop int, suffix string) string {
				var result []string
				for i := start; i <= stop; i++ {
					result = append(result, fmt.Sprintf("%d %s", i, suffix))
				}
				return strings.Join(result, "\n")
			}

			Context("when stdout and stderr contains more than 100 lines", func() {
				It("returns error message with truncated stdout and stderr to 100 lines", func() {
					fullStdout101 := buildLines(1, 101, "stdout")
					truncatedStdout100 := buildLines(2, 101, "stdout")

					fullStderr101 := buildLines(1, 101, "stderr")
					truncatedStderr100 := buildLines(2, 101, "stderr")

					execErr := NewExecError("fake-cmd", fullStdout101, fullStderr101)

					expectedMsg := fmt.Sprintf(
						"Running command: 'fake-cmd', stdout: '%s', stderr: '%s'",
						truncatedStdout100, truncatedStderr100,
					)

					Expect(execErr.ShortError()).To(Equal(expectedMsg))
				})
			})

			Context("when stdout and stderr contains exactly 100 lines", func() {
				It("returns error message with full lines", func() {
					stdout100 := buildLines(1, 100, "stdout")
					stderr100 := buildLines(1, 100, "stderr")
					execErr := NewExecError("fake-cmd", stdout100, stderr100)
					expectedMsg := fmt.Sprintf("Running command: 'fake-cmd', stdout: '%s', stderr: '%s'", stdout100, stderr100)
					Expect(execErr.ShortError()).To(Equal(expectedMsg))
				})
			})

			Context("when stdout and stderr contains less than 100 lines", func() {
				It("returns error message with full lines", func() {
					stdout99 := buildLines(1, 99, "stdout")
					stderr99 := buildLines(1, 99, "stderr")
					execErr := NewExecError("fake-cmd", stdout99, stderr99)
					expectedMsg := fmt.Sprintf("Running command: 'fake-cmd', stdout: '%s', stderr: '%s'", stdout99, stderr99)
					Expect(execErr.ShortError()).To(Equal(expectedMsg))
				})
			})
		})
	})
}
