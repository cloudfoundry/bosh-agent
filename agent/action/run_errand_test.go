package action_test

import (
	"errors"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	boshenv "github.com/cloudfoundry/bosh-agent/v2/agent/script/pathenv"
)

var _ = Describe("RunErrand", func() {
	var (
		specService     *fakeas.FakeV1Service
		cmdRunner       *fakesys.FakeCmdRunner
		runErrandAction action.RunErrandAction
		errandName      string
		fullCommand     string
	)

	BeforeEach(func() {
		specService = fakeas.NewFakeV1Service()
		cmdRunner = fakesys.NewFakeCmdRunner()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		runErrandAction = action.NewRunErrand(specService, "/fake-jobs-dir", cmdRunner, logger)
		errandName = "fake-job-name"
		if runtime.GOOS == "windows" {
			fullCommand = "powershell /fake-jobs-dir/fake-job-name/bin/run"
		} else {
			fullCommand = "/fake-jobs-dir/fake-job-name/bin/run"
		}
	})

	AssertActionIsAsynchronous(runErrandAction)
	AssertActionIsNotPersistent(runErrandAction)
	AssertActionIsLoggable(runErrandAction)

	AssertActionIsNotResumable(runErrandAction)

	Describe("Run", func() {
		Context("when apply spec is successfully retrieved", func() {
			Context("when working with the old director that does not pass in errand name", func() {
				BeforeEach(func() {
					currentSpec := boshas.V1ApplySpec{}
					currentSpec.JobSpec.Template = "fake-job-name"
					specService.Spec = currentSpec
					cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
						WaitResult: boshsys.Result{
							Stdout:     "fake-stdout",
							Stderr:     "fake-stderr",
							ExitStatus: 0,
						},
					})
				})

				It("returns errand result without error after running an errand", func() {
					result, err := runErrandAction.Run()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(
						action.ErrandResult{
							Stdout:     "fake-stdout",
							Stderr:     "fake-stderr",
							ExitStatus: 0,
						},
					))
				})
			})

			Context("when current agent has a job spec template", func() {
				BeforeEach(func() {
					currentSpec := boshas.V1ApplySpec{}
					currentSpec.JobSpec.JobTemplateSpecs = []boshas.JobTemplateSpec{
						boshas.JobTemplateSpec{
							Version: "v1",
							Name:    "first-job"},
						boshas.JobTemplateSpec{
							Version: "v1",
							Name:    "fake-job-name"},
					}
					specService.Spec = currentSpec
				})

				Context("when errand script exits with non-0 exit code (execution of script is ok)", func() {
					BeforeEach(func() {
						cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								Stdout:     "fake-stdout",
								Stderr:     "fake-stderr",
								ExitStatus: 0,
							},
						})
					})

					It("returns errand result without error after running an errand", func() {
						result, err := runErrandAction.Run(errandName)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(
							action.ErrandResult{
								Stdout:     "fake-stdout",
								Stderr:     "fake-stderr",
								ExitStatus: 0,
							},
						))
					})

					It("runs errand script with properly configured environment", func() {
						_, err := runErrandAction.Run(errandName)
						Expect(err).ToNot(HaveOccurred())
						cmd := cmdRunner.RunComplexCommands[0]
						env := map[string]string{"PATH": boshenv.Path()}
						Expect(cmd.Env).To(Equal(env))
					})
				})

				Context("when errand script fails with non-0 exit code (execution of script is ok)", func() {
					BeforeEach(func() {
						cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								Stdout:     "fake-stdout",
								Stderr:     "fake-stderr",
								ExitStatus: 123,
								Error:      errors.New("fake-bosh-error"), // not used
							},
						})
					})

					It("returns errand result without an error", func() {
						result, err := runErrandAction.Run(errandName)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(
							action.ErrandResult{
								Stdout:     "fake-stdout",
								Stderr:     "fake-stderr",
								ExitStatus: 123,
							},
						))
					})
				})

				Context("when errand script fails to execute", func() {
					BeforeEach(func() {
						cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: -1,
								Error:      errors.New("fake-bosh-error"),
							},
						})
					})

					It("returns error because script failed to execute", func() {
						result, err := runErrandAction.Run(errandName)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-bosh-error"))
						Expect(result).To(Equal(action.ErrandResult{}))
					})
				})
			})

			Context("when current agent spec does not have a job spec template", func() {
				BeforeEach(func() {
					specService.Spec = boshas.V1ApplySpec{}
				})

				It("returns error stating the errand cannot be found", func() {
					_, err := runErrandAction.Run(errandName)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Could not find errand fake-job-name"))
				})

				It("does not run errand script", func() {
					_, err := runErrandAction.Run(errandName)
					Expect(err).To(HaveOccurred())
					Expect(len(cmdRunner.RunComplexCommands)).To(Equal(0))
				})
			})
		})

		Context("when apply spec could not be retrieved", func() {
			BeforeEach(func() {
				specService.GetErr = errors.New("fake-get-error")
			})

			It("returns error stating that job template is required", func() {
				_, err := runErrandAction.Run(errandName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-error"))
			})

			It("does not run errand script", func() {
				_, err := runErrandAction.Run(errandName)
				Expect(err).To(HaveOccurred())
				Expect(len(cmdRunner.RunComplexCommands)).To(Equal(0))
			})
		})
	})

	Describe("Cancel", func() {
		BeforeEach(func() {
			currentSpec := boshas.V1ApplySpec{
				JobSpec: boshas.JobSpec{
					JobTemplateSpecs: []boshas.JobTemplateSpec{
						boshas.JobTemplateSpec{
							Version: "v1",
							Name:    "first-job"},
						boshas.JobTemplateSpec{
							Version: "v1",
							Name:    "fake-job-name"},
					},
				},
			}
			specService.Spec = currentSpec
		})

		Context("when runErrandAction was not cancelled yet", func() {
			It("terminates errand nicely giving it 10 secs to exit on its own", func() {
				process := &fakesys.FakeProcess{
					TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
						p.WaitCh <- boshsys.Result{
							Stdout:     "fake-stdout",
							Stderr:     "fake-stderr",
							ExitStatus: 0,
						}
					},
				}

				cmdRunner.AddProcess(fullCommand, process)

				err := runErrandAction.Cancel()
				Expect(err).ToNot(HaveOccurred())

				_, err = runErrandAction.Run(errandName)
				Expect(err).ToNot(HaveOccurred())

				Expect(process.TerminateNicelyKillGracePeriod).To(Equal(10 * time.Second))
			})

			Context("when errand script exits with non-0 exit code (execution of script is ok)", func() {
				BeforeEach(func() {
					cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
						TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
							p.WaitCh <- boshsys.Result{
								Stdout:     "fake-stdout",
								Stderr:     "fake-stderr",
								ExitStatus: 0,
							}
						},
					})
				})

				It("returns errand result without error after running an errand", func() {
					err := runErrandAction.Cancel()
					Expect(err).ToNot(HaveOccurred())

					result, err := runErrandAction.Run(errandName)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(
						action.ErrandResult{
							Stdout:     "fake-stdout",
							Stderr:     "fake-stderr",
							ExitStatus: 0,
						},
					))
				})
			})

			Context("when errand script fails with non-0 exit code (execution of script is ok)", func() {
				BeforeEach(func() {
					cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
						TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
							p.WaitCh <- boshsys.Result{
								Stdout:     "fake-stdout",
								Stderr:     "fake-stderr",
								ExitStatus: 123,
								Error:      errors.New("fake-bosh-error"), // not used
							}
						},
					})
				})

				It("returns errand result without an error", func() {
					err := runErrandAction.Cancel()
					Expect(err).ToNot(HaveOccurred())

					result, err := runErrandAction.Run(errandName)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(
						action.ErrandResult{
							Stdout:     "fake-stdout",
							Stderr:     "fake-stderr",
							ExitStatus: 123,
						},
					))
				})
			})

			Context("when errand script fails to execute", func() {
				BeforeEach(func() {
					cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
						TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
							p.WaitCh <- boshsys.Result{
								ExitStatus: -1,
								Error:      errors.New("fake-bosh-error"),
							}
						},
					})
				})

				It("returns error because script failed to execute", func() {
					err := runErrandAction.Cancel()
					Expect(err).ToNot(HaveOccurred())

					result, err := runErrandAction.Run(errandName)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-bosh-error"))
					Expect(result).To(Equal(action.ErrandResult{}))
				})
			})
		})

		Context("when runErrandAction was cancelled already", func() {
			BeforeEach(func() {
				cmdRunner.AddProcess(fullCommand, &fakesys.FakeProcess{
					TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
						p.WaitCh <- boshsys.Result{
							ExitStatus: -1,
							Error:      errors.New("fake-bosh-error"),
						}
					},
				})
			})

			It("allows to cancel runErrandAction second time without returning an error", func() {
				err := runErrandAction.Cancel()
				Expect(err).ToNot(HaveOccurred())

				err = runErrandAction.Cancel()
				Expect(err).ToNot(HaveOccurred()) // returns without waiting
			})
		})
	})
})
