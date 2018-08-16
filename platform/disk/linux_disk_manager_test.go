package disk_test

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-agent/platform/disk"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewLinuxDiskManager", func() {
	var (
		ephemeralDiskPartitioned bool

		runner *fakesys.FakeCmdRunner
		fs     *fakesys.FakeFileSystem
		logger boshlog.Logger
	)

	BeforeEach(func() {
		ephemeralDiskPartitioned = false
		runner = fakesys.NewFakeCmdRunner()
		fs = fakesys.NewFakeFileSystem()
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	Context("when bindMount is set to false", func() {
		It("returns disk manager configured not to do bind mounting", func() {
			expectedMountsSearcher := disk.NewProcMountsSearcher(fs)
			expectedMounter := disk.NewLinuxMounter(runner, expectedMountsSearcher, 1*time.Second)

			diskManager := disk.NewLinuxDiskManager(logger, runner, fs, disk.LinuxDiskManagerOpts{}, ephemeralDiskPartitioned)
			Expect(diskManager.GetMounter()).To(Equal(expectedMounter))
		})
	})

	Context("when bindMount is set to true", func() {
		It("returns disk manager configured to do bind mounting", func() {
			expectedMountsSearcher := disk.NewCmdMountsSearcher(runner)
			expectedMounter := disk.NewLinuxBindMounter(disk.NewLinuxMounter(runner, expectedMountsSearcher, 1*time.Second))

			opts := disk.LinuxDiskManagerOpts{BindMount: true}
			diskManager := disk.NewLinuxDiskManager(logger, runner, fs, opts, ephemeralDiskPartitioned)
			Expect(diskManager.GetMounter()).To(Equal(expectedMounter))
		})
	})

	Context("when partitioner type is not set", func() {
		It("returns disk manager configured to use sfdisk", func() {
			opts := disk.LinuxDiskManagerOpts{}
			diskManager := disk.NewLinuxDiskManager(logger, runner, fs, opts, ephemeralDiskPartitioned)
			Expect(diskManager.GetEphemeralDevicePartitioner()).To(Equal(disk.NewSfdiskPartitioner(logger, runner, clock.NewClock())))
		})
	})

	Context("when partitioner type is 'parted'", func() {
		It("returns disk manager configured to use parted", func() {
			opts := disk.LinuxDiskManagerOpts{PartitionerType: "parted"}
			diskManager := disk.NewLinuxDiskManager(logger, runner, fs, opts, ephemeralDiskPartitioned)
			Expect(diskManager.GetEphemeralDevicePartitioner()).To(Equal(disk.NewPartedPartitioner(logger, runner, clock.NewClock(), ephemeralDiskPartitioned)))
		})

		It("returns disk manager configured to use parted with partitioned ephemeral disk", func() {
			ephemeralDiskPartitioned = true
			opts := disk.LinuxDiskManagerOpts{PartitionerType: "parted"}
			diskManager := disk.NewLinuxDiskManager(logger, runner, fs, opts, ephemeralDiskPartitioned)
			Expect(diskManager.GetEphemeralDevicePartitioner()).To(Equal(disk.NewPartedPartitioner(logger, runner, clock.NewClock(), ephemeralDiskPartitioned)))
		})
	})

	Context("when partitioner type is unknown", func() {
		It("panics", func() {
			opts := disk.LinuxDiskManagerOpts{PartitionerType: "unknown"}
			Expect(func() { disk.NewLinuxDiskManager(logger, runner, fs, opts, ephemeralDiskPartitioned) }).To(Panic())
		})
	})

	Context("GetPersistentDevicePartitioner", func() {
		var (
			mounter     disk.Mounter
			diskManager disk.Manager
		)

		BeforeEach(func() {
			diskManager = disk.NewLinuxDiskManager(logger, runner, fs, disk.LinuxDiskManagerOpts{}, ephemeralDiskPartitioned)
			mountsSearcher := disk.NewProcMountsSearcher(fs)
			mounter = disk.NewLinuxMounter(runner, mountsSearcher, 1*time.Second)
		})

		It("returns the default persistent disk partitioner", func() {
			partitioner, err := diskManager.GetPersistentDevicePartitioner("")
			Expect(err).NotTo(HaveOccurred())
			Expect(partitioner).To(Equal(disk.NewPersistentDevicePartitioner(
				disk.NewSfdiskPartitioner(logger, runner, clock.NewClock()),
				disk.NewPartedPartitioner(logger, runner, clock.NewClock(), ephemeralDiskPartitioned),
				disk.NewUtil(runner, mounter, fs, logger),
				logger,
			)))
		})

		Context("when parted is requested", func() {
			It("returns the parted partitioner", func() {
				partitioner, err := diskManager.GetPersistentDevicePartitioner("parted")
				Expect(err).NotTo(HaveOccurred())
				Expect(partitioner).To(Equal(disk.NewPartedPartitioner(logger, runner, clock.NewClock(), ephemeralDiskPartitioned)))
			})
		})

		Context("when an invalid partitioner is requested", func() {
			It("returns an error", func() {
				_, err := diskManager.GetPersistentDevicePartitioner("invalid")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
