package disk_test

import (
	"errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeboshaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("EphemeralDevicePartitioner", func() {
	var (
		devicePath string

		partitions []Partition

		fakeCmdRunner *fakesys.FakeCmdRunner
		fakefs        *fakesys.FakeFileSystem
		fakeclock     *fakeboshaction.FakeClock
		logger        boshlog.Logger

		partitioner Partitioner
	)

	BeforeEach(func() {
		devicePath = "/dev/edx"
		partitions = []Partition{
			{
				NamePrefix:  "fake-agent-id",
				SizeInBytes: 8589934592, // (8GiB)
				Type:        PartitionTypeLinux,
			},
			{
				NamePrefix:  "fake-agent-id",
				SizeInBytes: 8589934592, // (8GiB)
				Type:        PartitionTypeSwap,
			},
		}

		fakeCmdRunner = fakesys.NewFakeCmdRunner()
		fakefs = fakesys.NewFakeFileSystem()
		fakefs.WriteFile("/setting/path.json", []byte(`{
								"agent_id":"fake-agent-id"
							}`))

		logger = boshlog.NewLogger(boshlog.LevelNone)
		fakeCmdRunner = fakesys.NewFakeCmdRunner()
		fakeclock = &fakeboshaction.FakeClock{}
		partedPartitioner := NewPartedPartitioner(logger, fakeCmdRunner, fakeclock)

		partitioner = NewEphemeralDevicePartitioner(partedPartitioner, logger, fakeCmdRunner)
	})

	Describe("Partition", func() {
		Context("when there are no partitions", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/edx unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
				)
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/edx unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
				)
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/edx unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
				)
			})

			It("creates partitions using parted starting at the 1048576 byte", func() {
				err := partitioner.Partition(devicePath, partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(12))

				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-0", "1048576", "8590983167"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-1", "8590983168", "17180917759"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"partprobe", "/dev/edx"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"udevadm", "settle"}))
			})
		})

		Context("when the desired partitions do not exist", func() {
			Context("when there are existing partitions", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
1:512B:2048576B:199680B:linux-swap(v1):fake-agent-id-0:;
`},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
					)
				})

				It("recreates partitions using parted starting at the 1048576 byte", func() {
					partitions = []Partition{
						{
							NamePrefix:  "fake-agent-id",
							SizeInBytes: 17179869184, // (16GiB)
							Type:        PartitionTypeSwap,
						},
						{
							NamePrefix:  "fake-agent-id",
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeLinux,
						},
					}
					err := partitioner.Partition(devicePath, partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(12))

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "-a", "/dev/edx"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-0", "1048576", "17180917759"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-1", "17180917760", "25770852351"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"partprobe", "/dev/edx"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"udevadm", "settle"}))
				})
			})

			Context("when there are msdos paritions", func() {
				BeforeEach(func() {

					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:msdos:Xen Virtual Block Device:;
1:512B:8406236159B:8406235648B:linux-swap(v1)::;
2:8406236160B:107372805119B:98966568960B:ext4::;
`},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
					)
				})

				It("checks the existing partitions and does nothing", func() {

					partitions = []Partition{
						{
							NamePrefix:  "fake-agent-id",
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeSwap,
						},
						{
							NamePrefix:  "fake-agent-id",
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeLinux,
						},
					}
					err := partitioner.Partition(devicePath, partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(12))

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "-a", "/dev/edx"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-0", "1048576", "8590983167"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-1", "8590983168", "17180917759"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"partprobe", "/dev/edx"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"udevadm", "settle"}))
				})
			})
		})

		Context("when the existing partitions match desired partitions", func() {
			Context("when agent ID is not changed", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
1:512B:8589935104B:8589934592B:linux-swap(v1):fake-agent-id-0:;
2:8589935105B:17179869697B:8589934592B:ext4:fake-agent-id-1:;
`},
					)
				})

				It("checks the existing partitions and does nothing", func() {
					partitions = []Partition{
						{
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeSwap,
						},
						{
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeLinux,
						},
					}
					err := partitioner.Partition(devicePath, partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(2))

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
				})
			})

			Context("when agent ID is changed", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
1:512B:8589935104B:8589934592B:linux-swap(v1):old-agent-id-0:;
2:8589935105B:17179869697B:8589934592B:ext4:old-agent-id-1:;
`},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/edx unit B print",
						fakesys.FakeCmdResult{
							Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
					)
				})

				It("recreates partitions using parted starting at the 1048576 byte", func() {
					partitions = []Partition{
						{
							NamePrefix:  "fake-agent-id",
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeSwap,
						},
						{
							NamePrefix:  "fake-agent-id",
							SizeInBytes: 8589934592, // (8GiB)
							Type:        PartitionTypeLinux,
						},
					}
					err := partitioner.Partition(devicePath, partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(12))

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "-a", "/dev/edx"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-0", "1048576", "8590983167"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-1", "8590983168", "17180917759"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"partprobe", "/dev/edx"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"udevadm", "settle"}))
				})
			})
		})

		Context("when getting existing partitions returns an error", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/edx unit B print",
					fakesys.FakeCmdResult{
						Stdout: "Some weird error", ExitStatus: 1, Error: errors.New("Some weird error")},
				)
			})

			It("throw an error", func() {
				partitions := []Partition{
					{SizeInBytes: 8589934592}, // (8GiB)
					{SizeInBytes: 8589934592}, // (8GiB)
				}

				err := partitioner.Partition(devicePath, partitions)
				Expect(err).To(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(2))
				Expect(err.Error()).To(ContainSubstring("Getting existing partitions of `/dev/edx'"))
			})
		})

		Context("when removing existing partitions returns an error", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/edx unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
1:512B:2048576B:199680B:linux-swap(v1):fake-agent-id-0:;
`},
				)
				for i := 0; i < 20; i++ {
					fakeCmdRunner.AddCmdResult(
						"wipefs -a /dev/edx",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 2, Error: errors.New("fake-cmd-error")},
					)
				}
			})

			It("throw an error", func() {
				partitions = []Partition{
					{
						SizeInBytes: 17179869184, // (16GiB)
						Type:        PartitionTypeSwap,
					},
					{
						SizeInBytes: 8589934592, // (8GiB)
						Type:        PartitionTypeLinux,
					},
				}
				err := partitioner.Partition(devicePath, partitions)
				Expect(err).To(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(22))
				Expect(err.Error()).To(ContainSubstring("Removing existing partitions"))
			})
		})
	})

	Describe("GetDeviceSizeInBytes", func() {
		It("returns number in bytes (stripping newline) from lsblk", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/edx",
				fakesys.FakeCmdResult{Stdout: "123\n"},
			)

			num, err := partitioner.GetDeviceSizeInBytes(devicePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(num).To(Equal(uint64(123)))
		})

		It("returns error if lsblk fails", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/edx",
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)

			_, err := partitioner.GetDeviceSizeInBytes(devicePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-err"))
		})
	})
})
