package jobsupervisor_test

import (
	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"

	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/v2/agent/alert"
	"github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WrapperJobSupervisor", func() {

	var (
		fs             *fakesys.FakeFileSystem
		logger         boshlog.Logger
		dirProvider    boshdir.Provider
		fakeSupervisor *fakes.FakeJobSupervisor
		wrapper        JobSupervisor
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		err := fs.MkdirAll("/var/vcap/instance", 0666)
		Expect(err).NotTo(HaveOccurred())
		logger = boshlog.NewLogger(boshlog.LevelNone)
		dirProvider = boshdir.NewProvider("/var/vcap")

		fakeSupervisor = fakes.NewFakeJobSupervisor()

		wrapper = NewWrapperJobSupervisor(
			fakeSupervisor,
			fs,
			dirProvider,
			logger,
		)
	})

	It("Reload should delegate to the underlying job supervisor", func() {
		boomError := errors.New("BOOM")
		fakeSupervisor.ReloadErr = boomError
		err := wrapper.Reload()
		Expect(fakeSupervisor.Reloaded).To(BeTrue())
		Expect(err).To(Equal(boomError))
	})

	Describe("Start", func() {
		It("should delegate to the underlying job supervisor", func() {
			boomError := errors.New("BOOM")
			fakeSupervisor.StartErr = boomError
			err := wrapper.Start()
			Expect(fakeSupervisor.Started).To(BeTrue())
			Expect(err).To(Equal(boomError))
		})

		It("write the health json asynchronously", func() {
			fakeSupervisor.StatusStatus = "running"
			err := wrapper.Start()
			Expect(err).NotTo(HaveOccurred())

			healthFile := filepath.Join(dirProvider.InstanceDir(), "health.json")
			healthRaw, err := fs.ReadFile(healthFile)
			Expect(err).ToNot(HaveOccurred())
			health := &Health{}
			err = json.Unmarshal(healthRaw, health)
			Expect(err).NotTo(HaveOccurred())
			Expect(health.State).To(Equal("running"))
		})
	})

	It("Stop should delegate to the underlying job supervisor", func() {
		boomError := errors.New("BOOM")
		fakeSupervisor.StopErr = boomError
		err := wrapper.Stop()
		Expect(fakeSupervisor.Stopped).To(BeTrue())
		Expect(err).To(Equal(boomError))
	})

	It("StopAndWait should delegate to the underlying job supervisor", func() {
		boomError := errors.New("BOOM")
		fakeSupervisor.StopErr = boomError
		err := wrapper.StopAndWait()
		Expect(fakeSupervisor.StoppedAndWaited).To(BeTrue())
		Expect(err).To(Equal(boomError))
	})

	Describe("Unmointor", func() {
		It("Unmonitor should delegate to the underlying job supervisor", func() {
			boomError := errors.New("BOOM")
			fakeSupervisor.UnmonitorErr = boomError
			err := wrapper.Unmonitor()
			Expect(fakeSupervisor.Unmonitored).To(BeTrue())
			Expect(err).To(Equal(boomError))
		})

		It("write the health json asynchronously", func() {
			fakeSupervisor.StatusStatus = "stopped"
			_ = wrapper.Unmonitor()

			healthFile := filepath.Join(dirProvider.InstanceDir(), "health.json")
			healthRaw, err := fs.ReadFile(healthFile)
			Expect(err).ToNot(HaveOccurred())
			health := &Health{}
			err = json.Unmarshal(healthRaw, health)
			Expect(err).NotTo(HaveOccurred())
			Expect(health.State).To(Equal("stopped"))
		})

	})

	It("Status should delegate to the underlying job supervisor", func() {
		fakeSupervisor.StatusStatus = "my-status"
		status := wrapper.Status()
		Expect(status).To(Equal(fakeSupervisor.StatusStatus))
	})

	It("Processes should delegate to the underlying job supervisor", func() {
		fakeSupervisor.ProcessesStatus = []Process{
			{},
		}
		fakeSupervisor.ProcessesError = errors.New("BOOM")
		processes, err := wrapper.Processes()
		Expect(processes).To(Equal(fakeSupervisor.ProcessesStatus))
		Expect(err).To(Equal(fakeSupervisor.ProcessesError))
	})

	It("AddJob should delegate to the underlying job supervisor", func() {
		boomError := errors.New("BOOM")
		fakeSupervisor.StartErr = boomError
		_ = wrapper.AddJob("name", 0, "path")
		Expect(fakeSupervisor.AddJobArgs).To(Equal([]fakes.AddJobArgs{
			{
				Name:       "name",
				Index:      0,
				ConfigPath: "path",
			},
		}))
	})

	It("RemoveAllJobs should delegate to the underlying job supervisor", func() {
		fakeSupervisor.RemovedAllJobsErr = errors.New("BOOM")
		err := wrapper.RemoveAllJobs()
		Expect(fakeSupervisor.RemovedAllJobs).To(BeTrue())
		Expect(err).To(Equal(fakeSupervisor.RemovedAllJobsErr))
	})

	It("MonitorJobFailures should delegate to the underlying job supervisor", func() {
		var testAlert *alert.MonitAlert

		fakeSupervisor.JobFailureAlert = &alert.MonitAlert{ID: "test-alert"}
		_ = wrapper.MonitorJobFailures(func(a alert.MonitAlert) error {
			testAlert = &a

			return nil
		})
		Expect(testAlert).To(Equal(fakeSupervisor.JobFailureAlert))
	})
})
