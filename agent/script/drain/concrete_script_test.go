package drain_test

import (
	"errors"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	fakeaction "github.com/cloudfoundry/bosh-agent/v2/agent/action/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	. "github.com/cloudfoundry/bosh-agent/v2/agent/script/drain"
	"github.com/cloudfoundry/bosh-agent/v2/agent/script/drain/drainfakes"
	boshenv "github.com/cloudfoundry/bosh-agent/v2/agent/script/pathenv"
)

var _ = Describe("ConcreteScript", func() {
	var (
		fs                        *fakesys.FakeFileSystem
		runner                    *fakesys.FakeCmdRunner
		params                    ScriptParams
		fakeClock                 *fakeaction.FakeClock
		script                    ConcreteScript
		exampleSpec               func() applyspec.V1ApplySpec
		jobChangedFullCommand     string
		jobCheckStatusFullCommand string
		jobShutdownFullCommand    string
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		runner = fakesys.NewFakeCmdRunner()
		params = &drainfakes.FakeScriptParams{}
		fakeClock = &fakeaction.FakeClock{}
		if runtime.GOOS == "windows" {
			jobChangedFullCommand = "powershell /fake/script job_changed hash_unchanged bar foo"
			jobCheckStatusFullCommand = "powershell /fake/script job_check_status hash_unchanged"
			jobShutdownFullCommand = "powershell /fake/script job_shutdown hash_unchanged"
		} else {
			jobChangedFullCommand = "/fake/script job_changed hash_unchanged bar foo"
			jobCheckStatusFullCommand = "/fake/script job_check_status hash_unchanged"
			jobShutdownFullCommand = "/fake/script job_shutdown hash_unchanged"
		}
	})

	JustBeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		script = NewConcreteScript(fs, runner, "my-tag", "/fake/script", params, fakeClock, logger)
	})

	Describe("Tag", func() {
		It("returns path", func() {
			Expect(script.Tag()).To(Equal("my-tag"))
		})
	})

	Describe("Path", func() {
		It("returns path", func() {
			Expect(script.Path()).To(Equal("/fake/script"))
		})
	})

	Describe("Params", func() {
		It("returns params", func() {
			Expect(script.Params()).To(Equal(params))
		})
	})

	Describe("Exists", func() {
		It("returns bool", func() {
			Expect(script.Exists()).To(BeFalse())

			err := fs.WriteFile("/fake/script", []byte{})
			Expect(err).NotTo(HaveOccurred())
			Expect(script.Exists()).To(BeTrue())
		})
	})

	Describe("Run", func() {
		BeforeEach(func() {
			oldSpec := exampleSpec()
			newSpec := exampleSpec()

			s := newSpec.PackageSpecs["foo"]
			s.Sha1 = crypto.MustParseMultipleDigest("sha1:fooupdatedsha1")
			newSpec.PackageSpecs["foo"] = s

			s = newSpec.PackageSpecs["bar"]
			s.Sha1 = crypto.MustParseMultipleDigest("sha1:barupdatedsha1")
			newSpec.PackageSpecs["bar"] = s

			params = NewUpdateParams(oldSpec, newSpec)
		})

		It("runs drain script", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "1"}})

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())

			var expectedCmd boshsys.Command
			if runtime.GOOS == "windows" {
				expectedCmd = boshsys.Command{
					Name: "powershell",
					Args: []string{"/fake/script", "job_changed", "hash_unchanged", "bar", "foo"},
					Env: map[string]string{
						"PATH":                boshenv.Path(),
						"BOSH_JOB_STATE":      "{\"persistent_disk\":42}",
						"BOSH_JOB_NEXT_STATE": "{\"persistent_disk\":42}",
					},
				}
			} else {
				expectedCmd = boshsys.Command{
					Name: "/fake/script",
					Args: []string{"job_changed", "hash_unchanged", "bar", "foo"},
					Env: map[string]string{
						"PATH":                boshenv.Path(),
						"BOSH_JOB_STATE":      "{\"persistent_disk\":42}",
						"BOSH_JOB_NEXT_STATE": "{\"persistent_disk\":42}",
					},
				}
			}

			Expect(len(runner.RunComplexCommands)).To(Equal(1))
			Expect(runner.RunComplexCommands[0]).To(Equal(expectedCmd))
		})

		It("sets the PATH environment variable", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "1"}})
			Expect(script.Run()).To(Succeed())

			Expect(len(runner.RunComplexCommands)).To(Equal(1))
			Expect(runner.RunComplexCommands[0].Env).To(HaveKeyWithValue("PATH", boshenv.Path()))
		})

		It("sleeps when script returns a positive integer", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "12"}})

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.SleepCallCount()).To(Equal(1))
			Expect(fakeClock.SleepArgsForCall(0)).To(Equal(12 * time.Second))
		})

		It("sleeps then calls the script again as long as script returns a negative integer", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "-5"}})
			runner.AddProcess(jobCheckStatusFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "-5"}})
			runner.AddProcess(jobCheckStatusFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "-5"}})
			runner.AddProcess(jobCheckStatusFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "0"}})

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeClock.SleepCallCount()).To(Equal(4))
			Expect(fakeClock.SleepArgsForCall(0)).To(Equal(5 * time.Second))
			Expect(fakeClock.SleepArgsForCall(1)).To(Equal(5 * time.Second))
			Expect(fakeClock.SleepArgsForCall(2)).To(Equal(5 * time.Second))
			Expect(fakeClock.SleepArgsForCall(3)).To(Equal(0 * time.Second))
		})

		It("ignores whitespace in stdout", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "-56\n"}})
			runner.AddProcess(jobCheckStatusFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: " 0  \t\n"}})

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.SleepCallCount()).To(Equal(2))
			Expect(fakeClock.SleepArgsForCall(0)).To(Equal(56 * time.Second))
			Expect(fakeClock.SleepArgsForCall(1)).To(Equal(0 * time.Second))
		})

		It("returns error with non integer stdout", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "hello!"}})

			err := script.Run()
			Expect(err).To(HaveOccurred())
		})

		It("returns error when running command errors", func() {
			runner.AddProcess(jobChangedFullCommand,
				&fakesys.FakeProcess{WaitResult: boshsys.Result{Error: errors.New("woops")}})

			err := script.Run()
			Expect(err).To(HaveOccurred())
		})

		Describe("job state", func() {
			BeforeEach(func() {
				runner.AddProcess(jobChangedFullCommand,
					&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "1"}})
			})

			It("sets the BOSH_JOB_STATE env variable with info from current apply spec", func() {
				err := script.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(len(runner.RunComplexCommands)).To(Equal(1))

				env := runner.RunComplexCommands[0].Env
				Expect(env["BOSH_JOB_STATE"]).To(Equal("{\"persistent_disk\":42}"))
			})

			Context("when cannot get the job state", func() {
				BeforeEach(func() {
					fakeParams := &drainfakes.FakeScriptParams{}
					fakeParams.JobStateReturns("", errors.New("fake-job-state-err"))
					params = fakeParams
				})

				It("returns error  and does not run drain script", func() {
					err := script.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-job-state-err"))

					Expect(len(runner.RunComplexCommands)).To(Equal(0))
				})
			})
		})

		Describe("job next state", func() {
			BeforeEach(func() {
				commandResult := &fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "1"}}
				runner.AddProcess(jobChangedFullCommand, commandResult)
			})

			Context("when job next state is present", func() {
				It("sets the BOSH_JOB_NEXT_STATE env variable", func() {
					err := script.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(len(runner.RunComplexCommands)).To(Equal(1))
					Expect(runner.RunComplexCommands[0].Env["BOSH_JOB_NEXT_STATE"]).To(Equal("{\"persistent_disk\":42}"))
				})
			})

			Context("when job next state is empty", func() {
				BeforeEach(func() {
					params = NewShutdownParams(exampleSpec(), nil)

					runner.AddProcess(jobShutdownFullCommand,
						&fakesys.FakeProcess{WaitResult: boshsys.Result{Stdout: "1"}})
				})

				It("does not set the BOSH_JOB_NEXT_STATE env variable", func() {
					err := script.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(len(runner.RunComplexCommands)).To(Equal(1))
					Expect(runner.RunComplexCommands[0].Env).ToNot(HaveKey("BOSH_JOB_NEXT_STATE"))
				})
			})

			Context("when cannot get the job next state", func() {
				BeforeEach(func() {
					fakeParams := &drainfakes.FakeScriptParams{}
					fakeParams.JobNextStateReturns("", errors.New("fake-job-next-state-err"))
					params = fakeParams
				})

				It("returns error and does not run drain script", func() {
					err := script.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-job-next-state-err"))

					Expect(len(runner.RunComplexCommands)).To(Equal(0))
				})
			})
		})
	})

	Describe("Cancel", func() {
		BeforeEach(func() {
			oldSpec := exampleSpec()
			newSpec := exampleSpec()

			s := newSpec.PackageSpecs["foo"]
			s.Sha1 = crypto.MustParseMultipleDigest("sha1:fooupdatedsha1")
			newSpec.PackageSpecs["foo"] = s

			s = newSpec.PackageSpecs["bar"]
			s.Sha1 = crypto.MustParseMultipleDigest("sha1:barupdatedsha1")
			newSpec.PackageSpecs["bar"] = s

			params = NewUpdateParams(oldSpec, newSpec)
		})

		It("succeeds", func() {
			err := script.Cancel()
			Expect(err).ToNot(HaveOccurred())
		})

		It("doesn't block and succeeds when invoked twice", func() {
			err := script.Cancel()
			Expect(err).ToNot(HaveOccurred())
			err = script.Cancel()
			Expect(err).ToNot(HaveOccurred())
		})

		It("terminates script nicely", func() {
			process := &fakesys.FakeProcess{
				TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
					p.WaitCh <- boshsys.Result{
						Stdout:     "0",
						Stderr:     "fake-stderr",
						ExitStatus: 0,
					}
				},
			}
			runner.AddProcess(jobChangedFullCommand, process)

			err := script.Cancel()
			Expect(err).ToNot(HaveOccurred())
			err = script.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Script was cancelled by user request"))
			Expect(process.TerminatedNicely).To(BeTrue())
			Expect(process.TerminateNicelyKillGracePeriod).To(Equal(10 * time.Second))
		})

		It("waits for process to exit", func() {
			process := &fakesys.FakeProcess{
				TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
					p.WaitCh <- boshsys.Result{
						Stdout:     "",
						Stderr:     "Interrupted",
						ExitStatus: 137,
						Error:      errors.New("Interrupted"),
					}
				},
			}
			runner.AddProcess(jobChangedFullCommand, process)

			err := script.Cancel()
			Expect(err).ToNot(HaveOccurred())
			err = script.Run()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Script was cancelled by user request"))
			Expect(err.Error()).To(ContainSubstring("Interrupted"))
		})
	})

	exampleSpec = func() applyspec.V1ApplySpec {
		jobName := "foojob"
		return applyspec.V1ApplySpec{
			Deployment:                   "fake_deployment",
			ConfigurationHash:            "fake_deployment_config_hash",
			PersistentDisk:               42,
			RenderedTemplatesArchiveSpec: &applyspec.RenderedTemplatesArchiveSpec{},
			JobSpec: applyspec.JobSpec{
				Name:     &jobName,
				Release:  "fakerelease",
				Template: "jobtemplate",
				Version:  "jobtemplate_version",
				JobTemplateSpecs: []applyspec.JobTemplateSpec{
					{
						Name:    "jobtemplate",
						Version: "jobtemplate_version",
					},
				},
			},
			PackageSpecs: map[string]applyspec.PackageSpec{
				"foo": {
					Name:        "foo",
					Version:     "foo_version",
					Sha1:        crypto.MustParseMultipleDigest("sha1:foosha1"),
					BlobstoreID: "foo_blobid",
				},
				"bar": {
					Name:        "bar",
					Version:     "bar_version",
					Sha1:        crypto.MustParseMultipleDigest("sha1:barsha1"),
					BlobstoreID: "bar_blobid",
				},
			},
		}
	}
})
