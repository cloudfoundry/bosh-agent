//go:build !windows

package jobsupervisor_test

import (
	"errors"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	"github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
)

var _ = Describe("compositeJobSupervisor", func() {
	var (
		monitSupervisor   *fakes.FakeJobSupervisor
		systemdSupervisor *fakes.FakeJobSupervisor
		logger            boshlog.Logger
		composite         JobSupervisor
	)

	BeforeEach(func() {
		monitSupervisor = fakes.NewFakeJobSupervisor()
		systemdSupervisor = fakes.NewFakeJobSupervisor()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		composite = NewCompositeJobSupervisor(monitSupervisor, systemdSupervisor, logger)
	})

	Describe("AddJob", func() {
		It("routes a processes.yml path to the systemd supervisor", func() {
			err := composite.AddJob("my-job", 0, "/var/vcap/jobs/my-job/processes.yml")
			Expect(err).NotTo(HaveOccurred())
			Expect(systemdSupervisor.AddJobArgs).To(HaveLen(1))
			Expect(systemdSupervisor.AddJobArgs[0].ConfigPath).To(Equal("/var/vcap/jobs/my-job/processes.yml"))
			Expect(monitSupervisor.AddJobArgs).To(BeEmpty())
		})

		It("routes a monit path to the monit supervisor", func() {
			err := composite.AddJob("my-job", 0, "/var/vcap/jobs/my-job/monit")
			Expect(err).NotTo(HaveOccurred())
			Expect(monitSupervisor.AddJobArgs).To(HaveLen(1))
			Expect(monitSupervisor.AddJobArgs[0].ConfigPath).To(Equal("/var/vcap/jobs/my-job/monit"))
			Expect(systemdSupervisor.AddJobArgs).To(BeEmpty())
		})

		It("routes a .monit path to the monit supervisor", func() {
			err := composite.AddJob("my-job", 0, "/var/vcap/jobs/my-job/sub.monit")
			Expect(err).NotTo(HaveOccurred())
			Expect(monitSupervisor.AddJobArgs).To(HaveLen(1))
			Expect(systemdSupervisor.AddJobArgs).To(BeEmpty())
		})

		It("increments systemdJobCount for processes.yml", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/processes.yml")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			// Start is only called when systemdJobCount > 0
			systemdSupervisor.StartErr = errors.New("boom")
			err := composite.Start()
			Expect(err).To(HaveOccurred())
		})

		It("increments monitJobCount for monit paths", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			monitSupervisor.StartErr = errors.New("boom")
			err := composite.Start()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("RemoveAllJobs", func() {
		It("delegates to both supervisors and resets counts", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/processes.yml")
			_ = composite.AddJob("b", 0, "/jobs/b/monit")

			Expect(composite.RemoveAllJobs()).To(Succeed())
			Expect(monitSupervisor.RemovedAllJobs).To(BeTrue())
			Expect(systemdSupervisor.RemovedAllJobs).To(BeTrue())
		})

		It("returns an error if the monit supervisor fails", func() {
			monitSupervisor.RemovedAllJobsErr = errors.New("monit-remove-err")
			Expect(composite.RemoveAllJobs()).To(MatchError("monit-remove-err"))
		})

		It("returns an error if the systemd supervisor fails", func() {
			systemdSupervisor.RemovedAllJobsErr = errors.New("systemd-remove-err")
			Expect(composite.RemoveAllJobs()).To(MatchError("systemd-remove-err"))
		})
	})

	Describe("Start", func() {
		It("does nothing when no jobs are registered", func() {
			Expect(composite.Start()).To(Succeed())
			Expect(monitSupervisor.Started).To(BeFalse())
			Expect(systemdSupervisor.Started).To(BeFalse())
		})

		It("starts only monit when only monit jobs are registered", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			Expect(composite.Start()).To(Succeed())
			Expect(monitSupervisor.Started).To(BeTrue())
			Expect(systemdSupervisor.Started).To(BeFalse())
		})

		It("starts only systemd when only systemd jobs are registered", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/processes.yml")
			Expect(composite.Start()).To(Succeed())
			Expect(systemdSupervisor.Started).To(BeTrue())
			Expect(monitSupervisor.Started).To(BeFalse())
		})

		It("starts both when both types are registered", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			Expect(composite.Start()).To(Succeed())
			Expect(monitSupervisor.Started).To(BeTrue())
			Expect(systemdSupervisor.Started).To(BeTrue())
		})
	})

	Describe("Stop", func() {
		It("stops only the supervisors with registered jobs", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/processes.yml")
			Expect(composite.Stop()).To(Succeed())
			Expect(systemdSupervisor.Stopped).To(BeTrue())
			Expect(monitSupervisor.Stopped).To(BeFalse())
		})
	})

	Describe("Unmonitor", func() {
		It("unmonitors only supervisors with registered jobs", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			Expect(composite.Unmonitor()).To(Succeed())
			Expect(monitSupervisor.Unmonitored).To(BeTrue())
			Expect(systemdSupervisor.Unmonitored).To(BeFalse())
		})

		It("returns an error if monit unmonitor fails", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			monitSupervisor.UnmonitorErr = errors.New("monit-unmonitor-err")
			Expect(composite.Unmonitor()).To(MatchError("monit-unmonitor-err"))
		})
	})

	Describe("Status", func() {
		It("returns running when no jobs are registered", func() {
			Expect(composite.Status()).To(Equal("running"))
		})

		It("returns the monit status when only monit jobs are registered", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			monitSupervisor.StatusStatus = "failing"
			Expect(composite.Status()).To(Equal("failing"))
		})

		It("returns the systemd status when only systemd jobs are registered", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/processes.yml")
			systemdSupervisor.StatusStatus = "running"
			Expect(composite.Status()).To(Equal("running"))
		})

		It("returns starting if either delegate is starting", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			monitSupervisor.StatusStatus = "running"
			systemdSupervisor.StatusStatus = "starting"
			Expect(composite.Status()).To(Equal("starting"))
		})

		It("returns failing if either delegate is failing", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			monitSupervisor.StatusStatus = "running"
			systemdSupervisor.StatusStatus = "failing"
			Expect(composite.Status()).To(Equal("failing"))
		})

		It("returns stopped only when both delegates are stopped", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			monitSupervisor.StatusStatus = "stopped"
			systemdSupervisor.StatusStatus = "stopped"
			Expect(composite.Status()).To(Equal("stopped"))
		})

		It("returns running when both delegates are running", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			monitSupervisor.StatusStatus = "running"
			systemdSupervisor.StatusStatus = "running"
			Expect(composite.Status()).To(Equal("running"))
		})
	})

	Describe("Processes", func() {
		It("aggregates processes from both supervisors", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			_ = composite.AddJob("b", 0, "/jobs/b/processes.yml")
			monitSupervisor.ProcessesStatus = []Process{{Name: "monit-proc"}}
			systemdSupervisor.ProcessesStatus = []Process{{Name: "systemd-proc"}}

			procs, err := composite.Processes()
			Expect(err).NotTo(HaveOccurred())
			Expect(procs).To(ConsistOf(
				Process{Name: "monit-proc"},
				Process{Name: "systemd-proc"},
			))
		})

		It("returns only monit processes when no systemd jobs are registered", func() {
			_ = composite.AddJob("a", 0, "/jobs/a/monit")
			monitSupervisor.ProcessesStatus = []Process{{Name: "monit-proc"}}

			procs, err := composite.Processes()
			Expect(err).NotTo(HaveOccurred())
			Expect(procs).To(ConsistOf(Process{Name: "monit-proc"}))
		})
	})
})
