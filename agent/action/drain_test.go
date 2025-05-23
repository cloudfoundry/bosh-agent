package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	boshscript "github.com/cloudfoundry/bosh-agent/v2/agent/script"
	boshdrain "github.com/cloudfoundry/bosh-agent/v2/agent/script/drain"
	"github.com/cloudfoundry/bosh-agent/v2/agent/script/scriptfakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
	fakenotif "github.com/cloudfoundry/bosh-agent/v2/notification/fakes"
)

var _ = Describe("DrainAction", func() {
	var (
		notifier          *fakenotif.FakeNotifier
		specService       *fakeas.FakeV1Service
		jobScriptProvider *scriptfakes.FakeJobScriptProvider
		fakeScripts       map[string]*scriptfakes.FakeCancellableScript
		jobSupervisor     *fakejobsuper.FakeJobSupervisor
		drainAction       action.DrainAction
		logger            boshlog.Logger
	)

	BeforeEach(func() {
		fakeScripts = make(map[string]*scriptfakes.FakeCancellableScript)
		logger = boshlog.NewLogger(boshlog.LevelNone)
		notifier = fakenotif.NewFakeNotifier()
		specService = fakeas.NewFakeV1Service()
		jobScriptProvider = &scriptfakes.FakeJobScriptProvider{}
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		drainAction = action.NewDrain(notifier, specService, jobScriptProvider, jobSupervisor, logger)
	})

	BeforeEach(func() {
		jobScriptProvider.NewDrainScriptStub = func(jobName string, params boshdrain.ScriptParams) boshscript.CancellableScript {
			_, exists := fakeScripts[jobName]
			if !exists {
				fakeScripts[jobName] = &scriptfakes.FakeCancellableScript{}
			}
			return fakeScripts[jobName]
		}
	})

	AssertActionIsAsynchronous(drainAction)
	AssertActionIsNotPersistent(drainAction)
	AssertActionIsLoggable(drainAction)

	AssertActionIsNotResumable(drainAction)

	Describe("Run", func() {
		var (
			parallelScript *scriptfakes.FakeCancellableScript
		)

		BeforeEach(func() {
			parallelScript = &scriptfakes.FakeCancellableScript{}
			jobScriptProvider.NewParallelScriptReturns(parallelScript)
		})

		addJobTemplate := func(spec *boshas.JobSpec, name string) {
			spec.Template = name
			spec.JobTemplateSpecs = append(spec.JobTemplateSpecs, boshas.JobTemplateSpec{Name: name})
		}

		Context("when drain update is requested", func() {
			var newSpec boshas.V1ApplySpec

			BeforeEach(func() {
				newSpec = boshas.V1ApplySpec{
					PackageSpecs: map[string]boshas.PackageSpec{
						"foo": {
							Name: "foo",
							Sha1: crypto.MustParseMultipleDigest("sha1:foosha1new"),
						},
					},
					RenderedTemplatesArchiveSpec: &boshas.RenderedTemplatesArchiveSpec{},
				}
			})

			act := func() (int, error) {
				return drainAction.Run(action.DrainTypeUpdate, newSpec)
			}

			Context("when current agent has a job spec template", func() {
				var currentSpec boshas.V1ApplySpec

				BeforeEach(func() {
					currentSpec = boshas.V1ApplySpec{
						RenderedTemplatesArchiveSpec: &boshas.RenderedTemplatesArchiveSpec{},
					}

					addJobTemplate(&currentSpec.JobSpec, "foo")
					addJobTemplate(&currentSpec.JobSpec, "bar")

					specService.Spec = currentSpec
				})

				It("unmonitors services so that drain scripts can kill processes on their own", func() {
					value, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal(0))

					Expect(jobSupervisor.Unmonitored).To(BeTrue())
				})

				Context("when unmonitoring services succeeds", func() {
					It("does not notify of job shutdown", func() {
						value, err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(value).To(Equal(0))

						Expect(notifier.NotifiedShutdown).To(BeFalse())
					})

					Context("when new apply spec is provided", func() {
						It("runs drain script with update params in parallel", func() {
							fooScript := &scriptfakes.FakeCancellableScript{}
							fooScript.TagReturns("foo")

							barScript := &scriptfakes.FakeCancellableScript{}
							barScript.TagReturns("bar")

							jobScriptProvider.NewDrainScriptStub = func(jobName string, params boshdrain.ScriptParams) boshscript.CancellableScript {
								Expect(params).To(Equal(boshdrain.NewUpdateParams(currentSpec, newSpec)))

								if jobName == "foo" { //nolint:staticcheck
									return fooScript
								} else if jobName == "bar" {
									return barScript
								} else {
									panic("Non-matching update drain script created")
								}
							}

							parallelScript.RunReturns(nil)

							value, err := act()
							Expect(err).ToNot(HaveOccurred())
							Expect(value).To(Equal(0))

							Expect(parallelScript.RunCallCount()).To(Equal(1))
							Expect(jobScriptProvider.NewParallelScriptCallCount()).To(Equal(1))

							scriptName, scripts := jobScriptProvider.NewParallelScriptArgsForCall(0)
							Expect(scriptName).To(Equal("drain"))
							Expect(scripts).To(Equal([]boshscript.Script{fooScript, barScript}))
						})

						It("returns an error when parallel script fails", func() {
							parallelScript.RunReturns(errors.New("fake-error"))

							value, err := act()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-error"))
							Expect(value).To(Equal(0))
						})
					})

					Context("when apply spec is not provided", func() {
						It("returns error", func() {
							value, err := drainAction.Run(action.DrainTypeUpdate)
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("Drain update requires new spec"))
							Expect(value).To(Equal(0))
						})
					})
				})

				Context("when unmonitoring services fails", func() {
					It("returns error", func() {
						jobSupervisor.UnmonitorErr = errors.New("fake-unmonitor-error")

						value, err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-unmonitor-error"))
						Expect(value).To(Equal(0))
					})
				})
			})

			Context("when current agent spec does not have a job spec template", func() {
				It("returns 0 and does not run drain script", func() {
					specService.Spec = boshas.V1ApplySpec{}

					value, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal(0))

					Expect(jobScriptProvider.NewDrainScriptCallCount()).To(Equal(0))
				})
			})
		})

		Context("when drain shutdown is requested", func() {
			act := func() (int, error) { return drainAction.Run(action.DrainTypeShutdown) }

			Context("when current agent has a job spec template", func() {
				var (
					currentSpec boshas.V1ApplySpec
				)

				BeforeEach(func() {
					currentSpec = boshas.V1ApplySpec{
						RenderedTemplatesArchiveSpec: &boshas.RenderedTemplatesArchiveSpec{}, // todo
					}
					addJobTemplate(&currentSpec.JobSpec, "foo")
					addJobTemplate(&currentSpec.JobSpec, "bar")
					specService.Spec = currentSpec
				})

				It("unmonitors services so that drain scripts can kill processes on their own", func() {
					value, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal(0))

					Expect(jobSupervisor.Unmonitored).To(BeTrue())
				})

				Context("when unmonitoring services succeeds", func() {
					It("notifies that job is about to shutdown", func() {
						value, err := act()
						Expect(err).ToNot(HaveOccurred())
						Expect(value).To(Equal(0))

						Expect(notifier.NotifiedShutdown).To(BeTrue())
					})

					Context("when job shutdown notification succeeds", func() {
						It("runs drain script with shutdown params in parallel", func() {
							fooScript := &scriptfakes.FakeCancellableScript{}
							fooScript.TagReturns("foo")

							barScript := &scriptfakes.FakeCancellableScript{}
							barScript.TagReturns("bar")

							jobScriptProvider.NewDrainScriptStub = func(jobName string, params boshdrain.ScriptParams) boshscript.CancellableScript {
								Expect(params).To(Equal(boshdrain.NewShutdownParams(currentSpec, nil)))

								if jobName == "foo" { //nolint:staticcheck
									return fooScript
								} else if jobName == "bar" {
									return barScript
								} else {
									panic("Non-matching shutdown drain script created")
								}
							}

							parallelScript.RunReturns(nil)

							value, err := act()
							Expect(err).ToNot(HaveOccurred())
							Expect(value).To(Equal(0))

							Expect(parallelScript.RunCallCount()).To(Equal(1))
							Expect(jobScriptProvider.NewParallelScriptCallCount()).To(Equal(1))

							scriptName, scripts := jobScriptProvider.NewParallelScriptArgsForCall(0)
							Expect(scriptName).To(Equal("drain"))
							Expect(scripts).To(Equal([]boshscript.Script{fooScript, barScript}))
						})

						It("returns an error when parallel script fails", func() {
							parallelScript.RunReturns(errors.New("fake-error"))

							value, err := act()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-error"))
							Expect(value).To(Equal(0))
						})
					})

					Context("when job shutdown notification fails", func() {
						It("returns error if job shutdown notifications errs", func() {
							notifier.NotifyShutdownErr = errors.New("fake-shutdown-error")

							value, err := act()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("fake-shutdown-error"))
							Expect(value).To(Equal(0))
						})
					})
				})

				Context("when unmonitoring services fails", func() {
					It("returns error", func() {
						jobSupervisor.UnmonitorErr = errors.New("fake-unmonitor-error")

						value, err := act()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-unmonitor-error"))
						Expect(value).To(Equal(0))
					})
				})
			})

			Context("when current agent spec does not have a job spec template", func() {
				It("returns 0 and does not run drain script", func() {
					specService.Spec = boshas.V1ApplySpec{}

					value, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(value).To(Equal(0))

					Expect(jobScriptProvider.NewDrainScriptCallCount()).To(Equal(0))
				})
			})
		})

		Context("when drain status is requested", func() {
			act := func() (int, error) { return drainAction.Run(action.DrainTypeStatus) }

			It("returns an error", func() {
				value, err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unexpected call with drain type 'status'"))
				Expect(value).To(Equal(0))
			})

			It("does not unmonitor services ", func() {
				_, _ = act() //nolint:errcheck
				Expect(jobSupervisor.Unmonitored).To(BeFalse())
			})

			It("does not notify of job shutdown", func() {
				_, _ = act() //nolint:errcheck
				Expect(notifier.NotifiedShutdown).To(BeFalse())
			})
		})
	})

	Describe("Cancel", func() {
		var (
			parallelScript *scriptfakes.FakeCancellableScript
			newSpec        = boshas.V1ApplySpec{
				PackageSpecs: map[string]boshas.PackageSpec{
					"foo": {
						Name: "foo",
						Sha1: crypto.MustParseMultipleDigest("sha1:foosha1new"),
					},
				},
			}
		)

		BeforeEach(func() {
			parallelScript = &scriptfakes.FakeCancellableScript{}
			jobScriptProvider.NewDrainScriptStub = func(jobName string, params boshdrain.ScriptParams) boshscript.CancellableScript {
				return &scriptfakes.FakeCancellableScript{}
			}
			jobScriptProvider.NewParallelScriptReturns(parallelScript)
			currentSpec := boshas.V1ApplySpec{}
			specService.Spec = currentSpec
		})

		Context("when drainAction was not canceled yet", func() {
			It("cancel drainAction", func() {
				_, err := drainAction.Run(action.DrainTypeShutdown, newSpec)
				Expect(err).ToNot(HaveOccurred())

				err = drainAction.Cancel()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
