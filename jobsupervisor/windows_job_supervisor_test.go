package jobsupervisor_test

import (
	. "github.com/cloudfoundry/bosh-agent/jobsupervisor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WindowsJobSupervisor", func() {
	var (
		jobSupervisor JobSupervisor
	)

	BeforeEach(func() {
		jobSupervisor = NewWindowsJobSupervisor()
	})

	Context("when started", func() {
		BeforeEach(func() {
			jobSupervisor.Start()
		})

		It("reports running", func() {
			Expect(jobSupervisor.Status()).To(Equal("running"))
		})
	})

	Context("when stopped", func() {
		BeforeEach(func() {
			jobSupervisor.Stop()
		})

		It("reports stopped", func() {
			Expect(jobSupervisor.Status()).To(Equal("stopped"))
		})
	})

	Context("when unmonitored", func() {
		BeforeEach(func() {
			jobSupervisor.Unmonitor()
		})

		It("reports unmonitored", func() {
			Expect(jobSupervisor.Status()).To(Equal("unmonitored"))
		})
	})
})
