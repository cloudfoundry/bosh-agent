package jobsupervisor_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshalert "github.com/cloudfoundry/bosh-agent/v2/agent/alert"
	boshhandler "github.com/cloudfoundry/bosh-agent/v2/handler"
	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	fakembus "github.com/cloudfoundry/bosh-agent/v2/mbus/fakes"
)

var _ = Describe("dummyNatsJobSupervisor", func() {
	var (
		dummyNats JobSupervisor
		handler   *fakembus.FakeHandler
	)

	BeforeEach(func() {
		handler = &fakembus.FakeHandler{}
		dummyNats = NewDummyNatsJobSupervisor(handler)
	})

	Describe("MonitorJobFailures", func() {
		It("monitors job status", func() {
			err := dummyNats.MonitorJobFailures(func(boshalert.MonitAlert) error { return nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(handler.RegisteredAdditionalFunc).ToNot(BeNil())
		})
	})

	Describe("Status", func() {
		BeforeEach(func() {
			err := dummyNats.MonitorJobFailures(func(boshalert.MonitAlert) error { return nil })
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the received status", func() {
			statusMessage := boshhandler.NewRequest("", "set_dummy_status", []byte(`{"status":"failing"}`), 0)
			handler.RegisteredAdditionalFunc(statusMessage)
			Expect(dummyNats.Status()).To(Equal("failing"))
		})

		It("returns running as a default value", func() {
			Expect(dummyNats.Status()).To(Equal("running"))
		})

		It("does not change the status given other messages", func() {
			statusMessage := boshhandler.NewRequest("", "some_other_message", []byte(`{"status":"failing"}`), 0)
			handler.RegisteredAdditionalFunc(statusMessage)
			Expect(dummyNats.Status()).To(Equal("running"))
		})
	})

	Describe("Start", func() {
		BeforeEach(func() {
			err := dummyNats.MonitorJobFailures(func(boshalert.MonitAlert) error { return nil })
			Expect(err).NotTo(HaveOccurred())
		})

		Context("When set_task_fail flag is sent in messagae", func() {
			It("raises an error", func() {
				statusMessage := boshhandler.NewRequest("", "set_task_fail", []byte(`{"status":"fail_task"}`), 0)
				handler.RegisteredAdditionalFunc(statusMessage)
				err := dummyNats.Start()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-task-fail-error"))
			})
		})

		Context("when set_task_fail flag is not sent in message", func() {
			It("does not raise an error", func() {
				statusMessage := boshhandler.NewRequest("", "set_task_fail", []byte(`{"status":"something_else"}`), 0)
				handler.RegisteredAdditionalFunc(statusMessage)
				err := dummyNats.Start()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Stop", func() {
		It("changes status to 'stopped'", func() {
			err := dummyNats.Stop()
			Expect(err).ToNot(HaveOccurred())
			Expect(dummyNats.Status()).To(Equal("stopped"))
		})

		Context("when a job is failing", func() {
			BeforeEach(func() {
				err := dummyNats.MonitorJobFailures(func(boshalert.MonitAlert) error { return nil })
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with 'fail_task'", func() {
				It("does not change status", func() {
					statusMessage := boshhandler.NewRequest("", "set_task_fail", []byte(`{"status":"fail_task"}`), 0)
					handler.RegisteredAdditionalFunc(statusMessage)
					err := dummyNats.Stop()
					Expect(err).ToNot(HaveOccurred())
					Expect(dummyNats.Status()).To(Equal("fail_task"))
				})
			})

			Context("with 'failing'", func() {
				It("does not change status", func() {
					statusMessage := boshhandler.NewRequest("", "set_dummy_status", []byte(`{"status":"failing"}`), 0)
					handler.RegisteredAdditionalFunc(statusMessage)
					err := dummyNats.Stop()
					Expect(err).ToNot(HaveOccurred())
					Expect(dummyNats.Status()).To(Equal("failing"))
				})
			})
		})
	})

	Describe("StopAndWait", func() {
		It("changes status to 'stopped'", func() {
			err := dummyNats.StopAndWait()
			Expect(err).ToNot(HaveOccurred())
			Expect(dummyNats.Status()).To(Equal("stopped"))
		})

		Context("when a job is failing", func() {
			BeforeEach(func() {
				err := dummyNats.MonitorJobFailures(func(boshalert.MonitAlert) error { return nil })
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with 'fail_task'", func() {
				It("does not change status", func() {
					statusMessage := boshhandler.NewRequest("", "set_task_fail", []byte(`{"status":"fail_task"}`), 0)
					handler.RegisteredAdditionalFunc(statusMessage)
					err := dummyNats.StopAndWait()
					Expect(err).ToNot(HaveOccurred())
					Expect(dummyNats.Status()).To(Equal("fail_task"))
				})
			})

			Context("with 'failing'", func() {
				It("does not change status", func() {
					statusMessage := boshhandler.NewRequest("", "set_dummy_status", []byte(`{"status":"failing"}`), 0)
					handler.RegisteredAdditionalFunc(statusMessage)
					err := dummyNats.StopAndWait()
					Expect(err).ToNot(HaveOccurred())
					Expect(dummyNats.Status()).To(Equal("failing"))
				})
			})
		})
	})
})
