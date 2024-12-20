package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	fakeapplyspec "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	boshscript "github.com/cloudfoundry/bosh-agent/v2/agent/script"
	"github.com/cloudfoundry/bosh-agent/v2/agent/script/scriptfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("RunScript", func() {
	var (
		fakeJobScriptProvider *scriptfakes.FakeJobScriptProvider
		specService           *fakeapplyspec.FakeV1Service
		runScriptAction       action.RunScriptAction
		options               action.RunScriptOptions
	)

	BeforeEach(func() {
		fakeJobScriptProvider = &scriptfakes.FakeJobScriptProvider{}
		specService = fakeapplyspec.NewFakeV1Service()
		specService.Spec.RenderedTemplatesArchiveSpec = &applyspec.RenderedTemplatesArchiveSpec{}
		logger := boshlog.NewLogger(boshlog.LevelNone)
		runScriptAction = action.NewRunScript(fakeJobScriptProvider, specService, logger)
		options = action.RunScriptOptions{
			Env: map[string]string{
				"FOO": "foo",
			},
		}
	})

	AssertActionIsAsynchronous(runScriptAction)
	AssertActionIsNotPersistent(runScriptAction)
	AssertActionIsLoggable(runScriptAction)

	AssertActionIsNotResumable(runScriptAction)
	AssertActionIsNotCancelable(runScriptAction)

	Describe("Run", func() {
		act := func() (map[string]string, error) { return runScriptAction.Run("run-me", options) }

		Context("when current spec can be retrieved", func() {
			var parallelScript *scriptfakes.FakeCancellableScript

			BeforeEach(func() {
				parallelScript = &scriptfakes.FakeCancellableScript{}
				fakeJobScriptProvider.NewParallelScriptReturns(parallelScript)
			})

			createFakeJob := func(jobName string) {
				spec := applyspec.JobTemplateSpec{Name: jobName}
				specService.Spec.JobSpec.JobTemplateSpecs = append(specService.Spec.JobSpec.JobTemplateSpecs, spec)
			}

			It("runs specified job scripts in parallel", func() {
				createFakeJob("fake-job-1")
				script1 := &scriptfakes.FakeScript{}
				script1.TagReturns("fake-job-1")

				createFakeJob("fake-job-2")
				script2 := &scriptfakes.FakeScript{}
				script2.TagReturns("fake-job-2")

				fakeJobScriptProvider.NewScriptStub = func(jobName, scriptName string, scriptEnv map[string]string) boshscript.Script {
					Expect(scriptName).To(Equal("run-me"))
					Expect(scriptEnv["FOO"]).To(Equal("foo"))

					if jobName == "fake-job-1" {
						return script1
					} else if jobName == "fake-job-2" {
						return script2
					} else {
						panic("Non-matching script created")
					}
				}

				parallelScript.RunReturns(nil)

				results, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(Equal(map[string]string{}))

				Expect(parallelScript.RunCallCount()).To(Equal(1))

				scriptName, scripts := fakeJobScriptProvider.NewParallelScriptArgsForCall(0)
				Expect(scriptName).To(Equal("run-me"))
				Expect(scripts).To(Equal([]boshscript.Script{script1, script2}))
			})

			It("returns an error when parallel script fails", func() {
				parallelScript.RunReturns(errors.New("fake-error"))

				results, err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
				Expect(results).To(Equal(map[string]string{}))
			})
		})

		Context("when current spec cannot be retrieved", func() {
			It("without current spec", func() {
				specService.GetErr = errors.New("fake-spec-get-error")

				results, err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-spec-get-error"))
				Expect(results).To(Equal(map[string]string{}))
			})
		})
	})
})
