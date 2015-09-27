package drain_test

import (
	"errors"
	"time"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	fakeaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	. "github.com/cloudfoundry/bosh-agent/agent/drain"
	"github.com/cloudfoundry/bosh-agent/agent/drain/fakes"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("ConcreteScript", func() {
	var (
		fs          *fakesys.FakeFileSystem
		runner      *fakesys.FakeCmdRunner
		params      ScriptParams
		fakeClock   *fakeaction.FakeClock
		script      ConcreteScript
		exampleSpec func() applyspec.V1ApplySpec
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		runner = fakesys.NewFakeCmdRunner()
		params = &fakes.FakeScriptParams{}
		fakeClock = &fakeaction.FakeClock{}
	})

	JustBeforeEach(func() {
		script = NewConcreteScript(fs, runner, "my-tag", "/fake/script", params, fakeClock)
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

			fs.WriteFile("/fake/script", []byte{})
			Expect(script.Exists()).To(BeTrue())
		})
	})

	Describe("Run", func() {
		BeforeEach(func() {
			oldSpec := exampleSpec()
			newSpec := exampleSpec()

			s := newSpec.PackageSpecs["foo"]
			s.Sha1 = "foo_updated_sha1"
			newSpec.PackageSpecs["foo"] = s

			s = newSpec.PackageSpecs["bar"]
			s.Sha1 = "bar_updated_sha1"
			newSpec.PackageSpecs["bar"] = s

			params = NewUpdateParams(oldSpec, newSpec)
		})

		It("runs drain script", func() {
			commandResult := fakesys.FakeCmdResult{Stdout: "1"}
			runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", commandResult)

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())

			expectedCmd := boshsys.Command{
				Name: "/fake/script",
				Args: []string{"job_unchanged", "hash_unchanged", "bar", "foo"},
				Env: map[string]string{
					"PATH":                "/usr/sbin:/usr/bin:/sbin:/bin",
					"BOSH_JOB_STATE":      "{\"persistent_disk\":42}",
					"BOSH_JOB_NEXT_STATE": "{\"persistent_disk\":42}",
				},
			}

			Expect(len(runner.RunComplexCommands)).To(Equal(1))
			Expect(runner.RunComplexCommands[0]).To(Equal(expectedCmd))
		})

		It("sleeps when script returns a positive integer", func() {
			commandResult := fakesys.FakeCmdResult{Stdout: "12"}
			runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", commandResult)

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.SleepCallCount()).To(Equal(1))
			Expect(fakeClock.SleepArgsForCall(0)).To(Equal(12 * time.Second))
		})

		It("sleeps then calls the script again as long as script returns a negative integer", func() {
			runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", fakesys.FakeCmdResult{Stdout: "-5"})
			runner.AddCmdResult("/fake/script job_check_status hash_unchanged", fakesys.FakeCmdResult{Stdout: "-5"})
			runner.AddCmdResult("/fake/script job_check_status hash_unchanged", fakesys.FakeCmdResult{Stdout: "-5"})
			runner.AddCmdResult("/fake/script job_check_status hash_unchanged", fakesys.FakeCmdResult{Stdout: "0"})

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.SleepCallCount()).To(Equal(4))
			Expect(fakeClock.SleepArgsForCall(0)).To(Equal(5 * time.Second))
			Expect(fakeClock.SleepArgsForCall(1)).To(Equal(5 * time.Second))
			Expect(fakeClock.SleepArgsForCall(2)).To(Equal(5 * time.Second))
			Expect(fakeClock.SleepArgsForCall(3)).To(Equal(0 * time.Second))
		})

		It("ignores whitespace in stdout", func() {
			runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", fakesys.FakeCmdResult{Stdout: "-56\n"})
			runner.AddCmdResult("/fake/script job_check_status hash_unchanged", fakesys.FakeCmdResult{Stdout: " 0  \t\n"})

			err := script.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeClock.SleepCallCount()).To(Equal(2))
			Expect(fakeClock.SleepArgsForCall(0)).To(Equal(56 * time.Second))
			Expect(fakeClock.SleepArgsForCall(1)).To(Equal(0 * time.Second))
		})

		It("returns error with non integer stdout", func() {
			commandResult := fakesys.FakeCmdResult{Stdout: "hello!"}
			runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", commandResult)

			err := script.Run()
			Expect(err).To(HaveOccurred())
		})

		It("returns error when running command errors", func() {
			commandResult := fakesys.FakeCmdResult{Error: errors.New("woops")}
			runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", commandResult)

			err := script.Run()
			Expect(err).To(HaveOccurred())
		})

		Describe("job state", func() {
			BeforeEach(func() {
				commandResult := fakesys.FakeCmdResult{Stdout: "1"}
				runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", commandResult)
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
					fakeParams := &fakes.FakeScriptParams{}
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
				commandResult := fakesys.FakeCmdResult{Stdout: "1"}
				runner.AddCmdResult("/fake/script job_unchanged hash_unchanged bar foo", commandResult)
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

					commandResult := fakesys.FakeCmdResult{Stdout: "1"}
					runner.AddCmdResult("/fake/script job_shutdown hash_unchanged", commandResult)
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
					fakeParams := &fakes.FakeScriptParams{}
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

	exampleSpec = func() applyspec.V1ApplySpec {
		jobName := "foojob"
		return applyspec.V1ApplySpec{
			Deployment:        "fake_deployment",
			ConfigurationHash: "fake_deployment_config_hash",
			PersistentDisk:    42,
			JobSpec: applyspec.JobSpec{
				Name:        &jobName,
				Release:     "fakerelease",
				Template:    "jobtemplate",
				Version:     "jobtemplate_version",
				Sha1:        "jobtemplate_sha1",
				BlobstoreID: "jobtemplate_blobid",
				JobTemplateSpecs: []applyspec.JobTemplateSpec{
					applyspec.JobTemplateSpec{
						Name:        "jobtemplate",
						Version:     "jobtemplate_version",
						Sha1:        "jobtemplate_sha1",
						BlobstoreID: "jobtemplate_blobid",
					},
				},
			},
			PackageSpecs: map[string]applyspec.PackageSpec{
				"foo": applyspec.PackageSpec{
					Name:        "foo",
					Version:     "foo_version",
					Sha1:        "foo_sha1",
					BlobstoreID: "foo_blobid",
				},
				"bar": applyspec.PackageSpec{
					Name:        "bar",
					Version:     "bar_version",
					Sha1:        "bar_sha1",
					BlobstoreID: "bar_blobid",
				},
			},
		}
	}
})
