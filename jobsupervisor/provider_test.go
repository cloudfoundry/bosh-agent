package jobsupervisor_test

import (
	"runtime"
	"time"

	. "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager/servicemanagerfakes"

	fakemonit "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/monit/fakes"
	fakembus "github.com/cloudfoundry/bosh-agent/v2/mbus/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func init() { //nolint:gochecknoinits
	Describe("provider", func() {
		var (
			platform   *platformfakes.FakePlatform
			fileSystem *fakesys.FakeFileSystem
			cmdRunner  *fakesys.FakeCmdRunner

			client                *fakemonit.FakeMonitClient
			logger                boshlog.Logger
			dirProvider           boshdir.Provider
			jobFailuresServerPort int
			handler               *fakembus.FakeHandler
			provider              Provider
			timeService           clock.Clock
			jobSupervisorName     string
			serviceManager        *servicemanagerfakes.FakeServiceManager
		)

		BeforeEach(func() {
			platform = &platformfakes.FakePlatform{}
			client = fakemonit.NewFakeMonitClient()
			fileSystem = fakesys.NewFakeFileSystem()
			cmdRunner = &fakesys.FakeCmdRunner{}
			logger = boshlog.NewLogger(boshlog.LevelNone)
			dirProvider = boshdir.NewProvider("/fake-base-dir")
			jobFailuresServerPort = 2825
			handler = &fakembus.FakeHandler{}
			timeService = clock.NewClock()
			serviceManager = &servicemanagerfakes.FakeServiceManager{}

			platform.GetFsReturns(fileSystem)
			platform.GetRunnerReturns(cmdRunner)

			provider = NewProvider(
				platform,
				client,
				logger,
				dirProvider,
				handler,
			)
			if runtime.GOOS == "windows" {
				jobSupervisorName = "windows"
			} else {
				jobSupervisorName = "monit"
			}
		})

		It("provides a monit/windows job supervisor", func() {
			actualSupervisor, err := provider.Get(jobSupervisorName)
			Expect(err).ToNot(HaveOccurred())

			if jobSupervisorName == "monit" {
				delegateSupervisor := NewMonitJobSupervisor(
					fileSystem,
					cmdRunner,
					client,
					logger,
					dirProvider,
					jobFailuresServerPort,
					MonitReloadOptions{
						MaxTries:               3,
						MaxCheckTries:          10,
						DelayBetweenCheckTries: 1 * time.Second,
					},
					timeService,
					serviceManager,
				)

				expectedSupervisor := NewWrapperJobSupervisor(
					delegateSupervisor,
					fileSystem,
					dirProvider,
					logger,
				)

				Expect(actualSupervisor).To(Equal(expectedSupervisor))
			}
		})

		It("provides a dummy job supervisor", func() {
			actualSupervisor, err := provider.Get("dummy")
			Expect(err).ToNot(HaveOccurred())

			expectedSupervisor := NewDummyJobSupervisor()
			Expect(actualSupervisor).To(Equal(expectedSupervisor))
		})

		It("provides a dummy nats job supervisor", func() {
			actualSupervisor, err := provider.Get("dummy-nats")
			Expect(err).NotTo(HaveOccurred())

			expectedSupervisor := NewDummyNatsJobSupervisor(handler)
			Expect(actualSupervisor).To(Equal(expectedSupervisor))
		})

		It("returns an error when the supervisor is not found", func() {
			_, err := provider.Get("does-not-exist")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does-not-exist could not be found"))
		})
	})
}
