package script_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	fakescript "github.com/cloudfoundry/bosh-agent/agent/script/fakes"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("ParallelScript", func() {
	var (
		scripts        []boshscript.Script
		parallelScript boshscript.ParallelScript
	)

	BeforeEach(func() {
		scripts = []boshscript.Script{}
	})

	JustBeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		parallelScript = boshscript.NewParallelScript("run-me", scripts, logger)
	})

	Describe("Run", func() {
		Context("when script exists", func() {
			var existingScript *fakescript.FakeScript

			BeforeEach(func() {
				existingScript = &fakescript.FakeScript{}
				existingScript.TagReturns("fake-job-1")
				existingScript.PathReturns("path/to/script1")
				existingScript.ExistsReturns(true)
				scripts = append(scripts, existingScript)
			})

			It("is executed", func() {
				existingScript.RunReturns(nil)

				results, err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(Equal(map[string]string{"fake-job-1": "executed"}))
			})

			It("gives an error when script fails", func() {
				existingScript.RunReturns(errors.New("fake-error"))

				results, err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(results).To(Equal(map[string]string{"fake-job-1": "failed"}))
			})
		})

		Context("when script does not exist", func() {
			var nonExistingScript *fakescript.FakeScript

			BeforeEach(func() {
				nonExistingScript = &fakescript.FakeScript{}
				nonExistingScript.ExistsReturns(false)
				scripts = append(scripts, nonExistingScript)
			})

			It("does not return a status for that script", func() {
				results, err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(Equal(map[string]string{}))
			})
		})

		Context("when running scripts concurrently", func() {
			var existingScript1 *fakescript.FakeScript
			var existingScript2 *fakescript.FakeScript

			BeforeEach(func() {
				existingScript1 = &fakescript.FakeScript{}
				existingScript1.TagReturns("fake-job-1")
				existingScript1.PathReturns("path/to/script1")
				existingScript1.ExistsReturns(true)
				scripts = append(scripts, existingScript1)

				existingScript2 = &fakescript.FakeScript{}
				existingScript2.TagReturns("fake-job-2")
				existingScript2.PathReturns("path/to/script2")
				existingScript2.ExistsReturns(true)
				scripts = append(scripts, existingScript2)
			})

			It("is executed and both scripts pass", func() {
				existingScript1.RunReturns(nil)
				existingScript2.RunReturns(nil)

				results, err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(Equal(map[string]string{"fake-job-1": "executed", "fake-job-2": "executed"}))
			})

			It("returns two failed statuses when both scripts fail", func() {
				existingScript1.RunReturns(errors.New("fake-error"))
				existingScript2.RunReturns(errors.New("fake-error"))

				results, err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("2 of 2 run-me script(s) failed. Failed Jobs:"))
				Expect(err.Error()).Should(ContainSubstring("fake-job-1"))
				Expect(err.Error()).Should(ContainSubstring("fake-job-2"))
				Expect(err.Error()).ShouldNot(ContainSubstring("Successful Jobs"))

				Expect(results).To(Equal(map[string]string{"fake-job-1": "failed", "fake-job-2": "failed"}))
			})

			It("returns one failed status when first script fail and second script pass, and when one fails continue waiting for unfinished tasks", func() {
				existingScript1.RunStub = func() error {
					time.Sleep(2 * time.Second)
					return errors.New("fake-error")
				}
				existingScript2.RunReturns(nil)

				results, err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("1 of 2 run-me script(s) failed. Failed Jobs: fake-job-1. Successful Jobs: fake-job-2."))
				Expect(results).To(Equal(map[string]string{"fake-job-1": "failed", "fake-job-2": "executed"}))
			})

			It("returns one failed status when first script pass and second script fail", func() {
				existingScript1.RunReturns(nil)
				existingScript2.RunReturns(errors.New("fake-error"))

				results, err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("1 of 2 run-me script(s) failed. Failed Jobs: fake-job-2. Successful Jobs: fake-job-1."))
				Expect(results).To(Equal(map[string]string{"fake-job-1": "executed", "fake-job-2": "failed"}))
			})

			It("wait for scripts to finish", func() {
				existingScript1.RunStub = func() error {
					time.Sleep(2 * time.Second)
					return nil
				}
				existingScript2.RunReturns(nil)

				results, err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(Equal(map[string]string{"fake-job-1": "executed", "fake-job-2": "executed"}))
			})

			It("runs the scripts concurrently", func(done Done) {
				waitGroup := &sync.WaitGroup{}
				waitGroup.Add(2)

				deadlockUnlessConcurrent := func() error {
					waitGroup.Done()
					waitGroup.Wait()
					return nil
				}

				existingScript1.RunStub = deadlockUnlessConcurrent
				existingScript2.RunStub = deadlockUnlessConcurrent

				_, err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(existingScript1.RunCallCount()).To(Equal(1))
				Expect(existingScript2.RunCallCount()).To(Equal(1))

				close(done)
			})
		})
	})
})
