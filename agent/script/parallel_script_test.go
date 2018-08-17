package script_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	"github.com/cloudfoundry/bosh-agent/agent/script/scriptfakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
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

	Describe("Tag", func() {
		It("returns empty string", func() {
			Expect(parallelScript.Tag()).To(Equal(""))
		})
	})

	Describe("Path", func() {
		It("returns empty string", func() {
			Expect(parallelScript.Path()).To(Equal(""))
		})
	})

	Describe("Exists", func() {
		It("returns true", func() {
			Expect(parallelScript.Exists()).To(BeTrue())
		})
	})

	Describe("Run", func() {
		Context("when there are no scripts", func() {
			BeforeEach(func() {
				scripts = []boshscript.Script{}
			})

			It("succeeds", func() {
				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when script exists", func() {
			var existingScript *scriptfakes.FakeScript

			BeforeEach(func() {
				existingScript = &scriptfakes.FakeScript{}
				existingScript.TagReturns("fake-job-1")
				existingScript.PathReturns("path/to/script1")
				existingScript.ExistsReturns(true)
				scripts = append(scripts, existingScript)
			})

			It("executes the script and succeeds", func() {
				existingScript.RunReturns(nil)

				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(existingScript.RunCallCount()).To(Equal(1))
			})

			It("gives an error when script fails", func() {
				existingScript.RunReturns(errors.New("fake-error"))

				err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("1 of 1 run-me scripts failed. Failed Jobs: fake-job-1."))

				Expect(existingScript.RunCallCount()).To(Equal(1))
			})
		})

		Context("when script does not exist", func() {
			var nonExistingScript *scriptfakes.FakeScript

			BeforeEach(func() {
				nonExistingScript = &scriptfakes.FakeScript{}
				nonExistingScript.ExistsReturns(false)
				scripts = append(scripts, nonExistingScript)
			})

			It("succeeds", func() {
				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when running scripts concurrently", func() {
			var existingScript1 *scriptfakes.FakeScript
			var existingScript2 *scriptfakes.FakeScript

			BeforeEach(func() {
				existingScript1 = &scriptfakes.FakeScript{}
				existingScript1.TagReturns("fake-job-1")
				existingScript1.PathReturns("path/to/script1")
				existingScript1.ExistsReturns(true)
				scripts = append(scripts, existingScript1)

				existingScript2 = &scriptfakes.FakeScript{}
				existingScript2.TagReturns("fake-job-2")
				existingScript2.PathReturns("path/to/script2")
				existingScript2.ExistsReturns(true)
				scripts = append(scripts, existingScript2)
			})

			It("executes the scripts and succeeds", func() {
				existingScript1.RunReturns(nil)
				existingScript2.RunReturns(nil)

				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(existingScript1.RunCallCount()).To(Equal(1))
				Expect(existingScript2.RunCallCount()).To(Equal(1))
			})

			It("returns two failed statuses when both scripts fail", func() {
				existingScript1.RunReturns(errors.New("fake-error"))
				existingScript2.RunReturns(errors.New("fake-error"))

				err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("2 of 2 run-me scripts failed. Failed Jobs:"))
				Expect(err.Error()).To(ContainSubstring("fake-job-1"))
				Expect(err.Error()).To(ContainSubstring("fake-job-2"))
				Expect(err.Error()).ToNot(ContainSubstring("Successful Jobs"))
			})

			It("returns one failed status when first script fail and second script pass, and when one fails continue waiting for unfinished tasks", func() {
				existingScript1.RunStub = func() error {
					time.Sleep(2 * time.Second)
					return errors.New("fake-error")
				}
				existingScript2.RunReturns(nil)

				err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("1 of 2 run-me scripts failed. Failed Jobs: fake-job-1. Successful Jobs: fake-job-2."))
			})

			It("returns one failed status when first script pass and second script fail", func() {
				existingScript1.RunReturns(nil)
				existingScript2.RunReturns(errors.New("fake-error"))

				err := parallelScript.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("1 of 2 run-me scripts failed. Failed Jobs: fake-job-2. Successful Jobs: fake-job-1."))
			})

			It("waits for scripts to finish", func() {
				existingScript1.RunStub = func() error {
					time.Sleep(2 * time.Second)
					return nil
				}
				existingScript2.RunReturns(nil)

				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(existingScript1.RunCallCount()).To(Equal(1))
				Expect(existingScript2.RunCallCount()).To(Equal(1))
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

				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())

				Expect(existingScript1.RunCallCount()).To(Equal(1))
				Expect(existingScript2.RunCallCount()).To(Equal(1))

				close(done)
			})
		})
	})

	Describe("Cancel", func() {
		Context("when there are no scripts", func() {
			BeforeEach(func() {
				scripts = []boshscript.Script{}
			})

			It("succeeds", func() {
				err := parallelScript.Cancel()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when script exists and is not cancelable", func() {
			var existingScript *scriptfakes.FakeScript

			BeforeEach(func() {
				existingScript = &scriptfakes.FakeScript{}
				existingScript.TagReturns("fake-job-1")
				existingScript.PathReturns("path/to/script1")
				existingScript.ExistsReturns(true)
				scripts = append(scripts, existingScript)
			})

			It("returns error", func() {
				existingScript.RunReturns(nil)
				err := parallelScript.Cancel()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when script exists and is cancelable", func() {
			BeforeEach(func() {
				scripts = append(scripts, &scriptfakes.FakeCancellableScript{})
			})

			It("succeeds", func() {
				err := parallelScript.Cancel()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when run cancelable scripts in parallel", func() {
			var existingScript1 *scriptfakes.FakeCancellableScript
			var existingScript2 *scriptfakes.FakeCancellableScript

			BeforeEach(func() {
				existingScript1 = &scriptfakes.FakeCancellableScript{}
				existingScript1.ExistsReturns(true)
				scripts = append(scripts, existingScript1)
				existingScript2 = &scriptfakes.FakeCancellableScript{}
				existingScript2.ExistsReturns(true)
				scripts = append(scripts, existingScript2)
			})

			It("succeeds", func() {
				err := parallelScript.Run()
				Expect(err).ToNot(HaveOccurred())
				err = parallelScript.Cancel()
				Expect(err).ToNot(HaveOccurred())
				Expect(existingScript1.CancelCallCount()).To(Equal(1))
				Expect(existingScript2.CancelCallCount()).To(Equal(1))
			})
		})
	})
})
