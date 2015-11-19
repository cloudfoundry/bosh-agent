package action_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bytes"
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakescript "github.com/cloudfoundry/bosh-agent/agent/script/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("RunErrand", func() {
	var (
		specService           *fakeas.FakeV1Service
		action                RunErrandAction
		fakeJobScriptProvider *fakescript.FakeJobScriptProvider
		fakeScript            *fakescript.FakeScript
	)

	BeforeEach(func() {
		fakeJobScriptProvider = &fakescript.FakeJobScriptProvider{}
		specService = fakeas.NewFakeV1Service()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		action = NewRunErrand(fakeJobScriptProvider, specService, "/fake-jobs-dir", logger)
		fakeScript = &fakescript.FakeScript{}
	})

	It("is asynchronous", func() {
		Expect(action.IsAsynchronous()).To(BeTrue())
	})

	It("is not persistent", func() {
		Expect(action.IsPersistent()).To(BeFalse())
	})

	Describe("Run", func() {
		Context("when apply spec is successfully retrieved", func() {
			Context("when current agent has a job spec template", func() {
				BeforeEach(func() {
					currentSpec := boshas.V1ApplySpec{}
					currentSpec.JobSpec.Template = "fake-job-name"
					specService.Spec = currentSpec
				})

				Context("when RunAsync returns an error", func() {
					BeforeEach(func() {
						fakeJobScriptProvider.NewScriptReturns(fakeScript)
						fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: 0,
							},
						}, nil, nil, errors.New("some-error"))
					})

					It("returns empty ErrandResult and wraps the error", func() {
						result, err := action.Run()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("some-error"))
						Expect(result).To(Equal(ErrandResult{}))
					})

				})

				Context("when errand script exits with 0 exit code (execution of script is ok)", func() {
					var stdout *fakesys.FakeFile
					var stderr *fakesys.FakeFile

					BeforeEach(func() {
						filesystem := fakesys.NewFakeFileSystem()
						filesystem.WriteFileString("/var/log/stdout.log", "test1")
						filesystem.WriteFileString("/var/log/stderr.log", "test2")

						stdout = fakesys.NewFakeFile("/var/log/stdout.log", filesystem)
						stderr = fakesys.NewFakeFile("/var/log/stderr.log", filesystem)
						fakeJobScriptProvider.NewScriptReturns(fakeScript)
						fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: 0,
							},
						}, stdout, stderr, nil)
					})

					It("returns errand result without error after running an errand", func() {
						result, err := action.Run()
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(
							ErrandResult{
								Stdout:     "test1",
								Stderr:     "test2",
								ExitStatus: 0,
							},
						))

						Eventually(stdout.Stats.Open).Should(BeFalse())
						Eventually(stderr.Stats.Open).Should(BeFalse())
					})
				})

				Context("when the stdout and stderr are larger than 10KB (each)", func() {

					var stdout *fakesys.FakeFile
					var stderr *fakesys.FakeFile

					BeforeEach(func() {
						var buffer bytes.Buffer

						for i := 0; i < 1300; i++ {
							buffer.WriteString("ABCDEFGHIJ")
						}
						tooMuchStrings := buffer.String()

						filesystem := fakesys.NewFakeFileSystem()
						filesystem.WriteFileString("/var/log/stdout.log", "beginning of stdout"+tooMuchStrings)
						filesystem.WriteFileString("/var/log/stderr.log", "beginning of stderr"+tooMuchStrings)

						stdout = fakesys.NewFakeFile("/var/log/stdout.log", filesystem)
						stderr = fakesys.NewFakeFile("/var/log/stderr.log", filesystem)
						fakeJobScriptProvider.NewScriptReturns(fakeScript)
						fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: 0,
							},
						}, stdout, stderr, nil)
					})

					It("should truncate stdstreams", func() {
						result, err := action.Run()
						Expect(err).ToNot(HaveOccurred())

						Expect(result.Stdout).To(ContainSubstring("<...log truncated...>"))
						Expect(result.Stderr).To(ContainSubstring("<...log truncated...>"))

						Expect(result.Stdout).ToNot(ContainSubstring("beginning of stdout"))
						Expect(result.Stderr).ToNot(ContainSubstring("beginning of stderr"))

						Expect(result.ExitStatus).To(Equal(0))
					})

				})

				Context("when errand script fails with non-0 exit code (execution of script is ok)", func() {
					var stdout *fakesys.FakeFile
					var stderr *fakesys.FakeFile

					BeforeEach(func() {
						filesystem := fakesys.NewFakeFileSystem()
						filesystem.WriteFileString("/var/log/stdout.log", "test1")
						filesystem.WriteFileString("/var/log/stderr.log", "test2")

						stdout = fakesys.NewFakeFile("/var/log/stdout.log", filesystem)
						stderr = fakesys.NewFakeFile("/var/log/stderr.log", filesystem)
						fakeJobScriptProvider.NewScriptReturns(fakeScript)
						fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: 123,
							},
						}, stdout, stderr, nil)
					})

					It("returns errand result without an error", func() {
						result, err := action.Run()
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(
							ErrandResult{
								Stdout:     "test1",
								Stderr:     "test2",
								ExitStatus: 123,
							},
						))

						Eventually(stdout.Stats.Open).Should(BeFalse())
						Eventually(stderr.Stats.Open).Should(BeFalse())
					})
				})

				Context("when errand script fails to execute", func() {

					BeforeEach(func() {
						fakeJobScriptProvider.NewScriptReturns(fakeScript)
						fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: -1,
								Error:      errors.New("fake-bosh-error"),
							},
						}, nil, nil, nil)
					})

					It("returns error because script failed to execute", func() {
						result, err := action.Run()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake-bosh-error"))
						Expect(result).To(Equal(ErrandResult{}))
					})
				})

				Context("when errand script stdout and stderr are nil", func() {
					BeforeEach(func() {
						fakeJobScriptProvider.NewScriptReturns(fakeScript)
						fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
							WaitResult: boshsys.Result{
								ExitStatus: 0,
							},
						}, nil, nil, nil)
					})

					It("should disclose that in the error returned", func() {
						result, err := action.Run()
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(
							ErrandResult{
								Stdout:     "Error retrieving logs",
								Stderr:     "Error retrieving logs",
								ExitStatus: 0,
							},
						))
					})
				})

			})

			Context("when current agent spec does not have a job spec template", func() {
				BeforeEach(func() {
					specService.Spec = boshas.V1ApplySpec{}
				})

				It("returns error stating that job template is required", func() {
					_, err := action.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("At least one job template is required to run an errand"))
				})

				It("does not run errand script", func() {
					_, err := action.Run()
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when apply spec could not be retrieved", func() {
			BeforeEach(func() {
				specService.GetErr = errors.New("fake-get-error")
			})

			It("returns error stating that job template is required", func() {
				_, err := action.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-error"))
			})

			It("does not run errand script", func() {
				_, err := action.Run()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Resume", func() {
		Context("When Resume is called", func() {
			It("returns a not supported error", func() {
				_, err := action.Resume()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not supported"))
			})
		})
	})

	Describe("Cancel", func() {
		BeforeEach(func() {
			fakeJobScriptProvider.NewScriptReturns(fakeScript)
			currentSpec := boshas.V1ApplySpec{}
			currentSpec.JobSpec.Template = "fake-job-name"
			specService.Spec = currentSpec
		})

		Context("when action was not cancelled yet", func() {
			It("terminates errand nicely giving it 10 secs to exit on its own", func() {
				process := &fakesys.FakeProcess{
					TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
						p.WaitCh <- boshsys.Result{
							ExitStatus: 0,
						}
					},
				}

				fakeScript.RunAsyncReturns(process, nil, nil, nil)

				err := action.Cancel()
				Expect(err).ToNot(HaveOccurred())

				_, err = action.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(process.TerminateNicelyKillGracePeriod).To(Equal(10 * time.Second))
			})

			Context("when errand script exits with non-0 exit code (execution of script is ok)", func() {
				var stdout *fakesys.FakeFile
				var stderr *fakesys.FakeFile

				BeforeEach(func() {
					filesystem := fakesys.NewFakeFileSystem()
					filesystem.WriteFileString("/var/log/stdout.log", "test1")
					filesystem.WriteFileString("/var/log/stderr.log", "test2")

					stdout = fakesys.NewFakeFile("/var/log/stdout.log", filesystem)
					stderr = fakesys.NewFakeFile("/var/log/stderr.log", filesystem)
					fakeJobScriptProvider.NewScriptReturns(fakeScript)
					fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
						TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
							p.WaitCh <- boshsys.Result{
								ExitStatus: 0,
							}
						},
					}, stdout, stderr, nil)

				})

				It("returns errand result without error after running an errand", func() {
					err := action.Cancel()
					Expect(err).ToNot(HaveOccurred())

					result, err := action.Run()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(
						ErrandResult{
							Stdout:     "test1",
							Stderr:     "test2",
							ExitStatus: 0,
						},
					))
				})
			})

			Context("when errand script fails with non-0 exit code (execution of script is ok)", func() {
				var stdout *fakesys.FakeFile
				var stderr *fakesys.FakeFile

				BeforeEach(func() {
					filesystem := fakesys.NewFakeFileSystem()
					filesystem.WriteFileString("/var/log/stdout.log", "test1")
					filesystem.WriteFileString("/var/log/stderr.log", "test2")

					stdout = fakesys.NewFakeFile("/var/log/stdout.log", filesystem)
					stderr = fakesys.NewFakeFile("/var/log/stderr.log", filesystem)
					fakeJobScriptProvider.NewScriptReturns(fakeScript)
					fakeScript.RunAsyncReturns(&fakesys.FakeProcess{
						TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
							p.WaitCh <- boshsys.Result{
								ExitStatus: 123,
								Error:      errors.New("fake-bosh-error"), // not used
							}
						},
					}, stdout, stderr, nil)
				})

				It("returns errand result without an error", func() {
					err := action.Cancel()
					Expect(err).ToNot(HaveOccurred())

					result, err := action.Run()
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(
						ErrandResult{
							Stdout:     "test1",
							Stderr:     "test2",
							ExitStatus: 123,
						},
					))
				})
			})

			Context("when errand script fails to execute (exit status of -1 and error returned)", func() {
				BeforeEach(func() {
					process := &fakesys.FakeProcess{
						TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
							p.WaitCh <- boshsys.Result{
								ExitStatus: -1,
								Error:      errors.New("fake-bosh-error"),
							}
						},
					}

					fakeScript.RunAsyncReturns(process, nil, nil, nil)
				})

				It("returns error because script failed to execute", func() {
					err := action.Cancel()
					Expect(err).ToNot(HaveOccurred())

					result, err := action.Run()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-bosh-error"))
					Expect(result).To(Equal(ErrandResult{}))
				})
			})
		})

		Context("when action was cancelled already", func() {
			BeforeEach(func() {
				process := &fakesys.FakeProcess{
					TerminatedNicelyCallBack: func(p *fakesys.FakeProcess) {
						p.WaitCh <- boshsys.Result{
							ExitStatus: -1,
							Error:      errors.New("fake-bosh-error"),
						}
					},
				}

				fakeScript.RunAsyncReturns(process, nil, nil, nil)
			})

			It("allows to cancel action second time without returning an error", func() {
				err := action.Cancel()
				Expect(err).ToNot(HaveOccurred())

				err = action.Cancel()
				Expect(err).ToNot(HaveOccurred()) // returns without waiting
			})
		})
	})
})
