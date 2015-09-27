package action_test

import (
	"errors"
	"sync"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/script/drain"
	fakedrain "github.com/cloudfoundry/bosh-agent/agent/script/drain/fakes"
	fakescript "github.com/cloudfoundry/bosh-agent/agent/script/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	fakenotif "github.com/cloudfoundry/bosh-agent/notification/fakes"
)

func addJobTemplate(spec *applyspec.JobSpec, name string) {
	spec.Template = name
	spec.JobTemplateSpecs = append(spec.JobTemplateSpecs, applyspec.JobTemplateSpec{Name: name})
}

func init() {
	Describe("DrainAction", func() {
		var (
			notifier          *fakenotif.FakeNotifier
			specService       *fakeas.FakeV1Service
			jobScriptProvider *fakescript.FakeJobScriptProvider
			fakeScripts       map[string]*fakedrain.FakeScript
			jobSupervisor     *fakejobsuper.FakeJobSupervisor
			action            DrainAction
			logger            boshlog.Logger
		)

		BeforeEach(func() {
			fakeScripts = make(map[string]*fakedrain.FakeScript)
			logger = boshlog.NewLogger(boshlog.LevelNone)
			notifier = fakenotif.NewFakeNotifier()
			specService = fakeas.NewFakeV1Service()
			jobScriptProvider = &fakescript.FakeJobScriptProvider{}
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			action = NewDrain(notifier, specService, jobScriptProvider, jobSupervisor, logger)
		})

		BeforeEach(func() {
			jobScriptProvider.NewDrainScriptStub = func(jobName string, params boshdrain.ScriptParams) boshscript.Script {
				_, exists := fakeScripts[jobName]
				if !exists {
					fakeScript := fakedrain.NewFakeScript(jobName)
					fakeScript.Params = params
					fakeScripts[jobName] = fakeScript
				}
				return fakeScripts[jobName]
			}
		})

		It("is asynchronous", func() {
			Expect(action.IsAsynchronous()).To(BeTrue())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		Context("when drain update is requested", func() {
			act := func() (int, error) { return action.Run(DrainTypeUpdate, boshas.V1ApplySpec{}) }

			Context("when current agent has a job spec template", func() {
				var currentSpec boshas.V1ApplySpec

				BeforeEach(func() {
					currentSpec = boshas.V1ApplySpec{}
					addJobTemplate(&currentSpec.JobSpec, "foo")
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
						newSpec := boshas.V1ApplySpec{
							PackageSpecs: map[string]boshas.PackageSpec{
								"foo": boshas.PackageSpec{
									Name: "foo",
									Sha1: "foo-sha1-new",
								},
							},
						}

						Context("when drain script exists", func() {
							It("runs drain script with job_shutdown param", func() {
								value, err := action.Run(DrainTypeUpdate, newSpec)
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								jobName, params := jobScriptProvider.NewDrainScriptArgsForCall(0)
								Expect(jobName).To(Equal("foo"))
								Expect(params).To(Equal(boshdrain.NewUpdateParams(currentSpec, newSpec)))
								Expect(fakeScripts["foo"].DidRun).To(BeTrue())
							})

							Context("when drain script runs and errs", func() {
								BeforeEach(func() {
									failingScript := fakedrain.NewFakeScript("foo")
									failingScript.RunError = errors.New("fake-drain-run-error")
									fakeScripts["foo"] = failingScript
								})

								It("returns error", func() {
									value, err := act()
									Expect(err).To(HaveOccurred())
									Expect(err.Error()).To(ContainSubstring("fake-drain-run-error"))
									Expect(value).To(Equal(0))
								})
							})
						})

						Context("when multiple job templates have drain scripts", func() {
							BeforeEach(func() {
								addJobTemplate(&currentSpec.JobSpec, "bar")
								specService.Spec = currentSpec
							})

							It("runs the scripts concurrently", func(done Done) {
								waitGroup := &sync.WaitGroup{}
								waitGroup.Add(2)

								deadlockUnlessConcurrent := func() error {
									waitGroup.Done()
									waitGroup.Wait()
									return nil
								}

								script1 := fakedrain.NewFakeScript("foo")
								script1.RunStub = deadlockUnlessConcurrent
								fakeScripts["foo"] = script1

								script2 := fakedrain.NewFakeScript("bar")
								script2.RunStub = deadlockUnlessConcurrent
								fakeScripts["bar"] = script2

								value, err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(script1.DidRun).To(BeTrue())
								Expect(script2.DidRun).To(BeTrue())

								close(done)
							})

							It("reports an error when any drain script fails", func() {
								failingScript := fakedrain.NewFakeScript("foo")
								failingScript.RunError = errors.New("fake-drain-run-error")
								fakeScripts["foo"] = failingScript

								workingScript := fakedrain.NewFakeScript("bar")
								fakeScripts["bar"] = workingScript

								value, err := act()
								Expect(err).To(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(failingScript.DidRun).To(BeTrue())
								Expect(workingScript.DidRun).To(BeTrue())
							})

							It("reports an success if all drain scripts succeed", func() {
								value, err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(fakeScripts["foo"].DidRun).To(BeTrue())
								Expect(fakeScripts["bar"].DidRun).To(BeTrue())
							})
						})

						Context("when drain script does not exist", func() {
							BeforeEach(func() {
								missingScript := fakedrain.NewFakeScript("foo")
								missingScript.ExistsBool = false
								fakeScripts["foo"] = missingScript
							})

							It("returns 0", func() {
								value, err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(fakeScripts["foo"].DidRun).To(BeFalse())
							})
						})
					})

					Context("when apply spec is not provided", func() {
						It("returns error", func() {
							value, err := action.Run(DrainTypeUpdate)
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
			act := func() (int, error) { return action.Run(DrainTypeShutdown) }

			Context("when current agent has a job spec template", func() {
				var currentSpec boshas.V1ApplySpec

				BeforeEach(func() {
					currentSpec = boshas.V1ApplySpec{}
					addJobTemplate(&currentSpec.JobSpec, "foo")
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
						Context("when drain script exists", func() {
							It("runs drain script with job_shutdown param passing no apply spec", func() {
								value, err := action.Run(DrainTypeShutdown)
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(fakeScripts["foo"].DidRun).To(BeTrue())
								Expect(fakeScripts["foo"].Params).To(Equal(boshdrain.NewShutdownParams(currentSpec, nil)))
							})

							It("runs drain script with job_shutdown param passing in first apply spec", func() {
								newSpec := boshas.V1ApplySpec{}
								addJobTemplate(&newSpec.JobSpec, "fake-updated-template")

								value, err := action.Run(DrainTypeShutdown, newSpec)
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(fakeScripts["foo"].DidRun).To(BeTrue())
								Expect(fakeScripts["foo"].Params).To(Equal(boshdrain.NewShutdownParams(currentSpec, &newSpec)))
							})

							Context("when drain script runs and errs", func() {
								BeforeEach(func() {
									failingScript := fakedrain.NewFakeScript("foo")
									failingScript.RunError = errors.New("fake-drain-run-error")
									fakeScripts["foo"] = failingScript
								})

								It("returns error", func() {
									value, err := act()
									Expect(err).To(HaveOccurred())
									Expect(err.Error()).To(ContainSubstring("fake-drain-run-error"))
									Expect(value).To(Equal(0))
								})
							})
						})

						Context("when drain script does not exist", func() {
							BeforeEach(func() {
								missingScript := fakedrain.NewFakeScript("foo")
								missingScript.ExistsBool = false
								fakeScripts["foo"] = missingScript
							})

							It("returns 0", func() {
								value, err := act()
								Expect(err).ToNot(HaveOccurred())
								Expect(value).To(Equal(0))

								Expect(fakeScripts["foo"].DidRun).To(BeFalse())
							})
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
			act := func() (int, error) { return action.Run(DrainTypeStatus) }

			It("returns an error", func() {
				value, err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unexpected call with drain type 'status'"))
				Expect(value).To(Equal(0))
			})
		})
	})
}
