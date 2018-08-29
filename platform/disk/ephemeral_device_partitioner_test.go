package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeboshaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	"github.com/cloudfoundry/bosh-agent/platform/disk/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("EphemeralDevicePartitioner", func() {
	var (
		devicePath string

		diskUtil   *fakes.FakeDiskUtil
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
		diskUtil = fakes.NewFakeDiskUtil()

		partitioner = NewEphemeralDevicePartitioner(partedPartitioner, diskUtil, logger, fakeCmdRunner, fakefs, fakeclock)
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
			})

			It("creates partitions using parted starting at the 1048576 byte", func() {
				err := partitioner.Partition(devicePath, partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(11))

				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"blkid"}))
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
						"blkid",
						fakesys.FakeCmdResult{
							Stdout: `/dev/xvda1: UUID="96dbf75b-3d78-4990-81e6-b8a5ce7c36f6" TYPE="ext4" PARTUUID="00057b93-01"
/dev/edx1: UUID="ae5f3f45-4f48-48ec-b3bd-c218b92e4a47" TYPE="swap" PARTLABEL="old-agent-id-0" PARTUUID="b5c66318-d96d-45c6-aebc-4b96823923a4"
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

					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(13))

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"blkid"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "-a", "/dev/edx1"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "/dev/edx", "rm", "1"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-0", "1048576", "17180917759"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/edx", "unit", "B", "mkpart", "fake-agent-id-1", "17180917760", "25770852351"}))
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
							SizeInBytes: 8589934592, // (16GiB)
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

			Context("when agent ID is not changed", func() {
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
						"blkid",
						fakesys.FakeCmdResult{
							Stdout: `/dev/xvda1: UUID="96dbf75b-3d78-4990-81e6-b8a5ce7c36f6" TYPE="ext4" PARTUUID="00057b93-01"
/dev/edx1: UUID="ae5f3f45-4f48-48ec-b3bd-c218b92e4a47" TYPE="swap" PARTLABEL="old-agent-id-0" PARTUUID="b5c66318-d96d-45c6-aebc-4b96823923a4"
/dev/edx2: UUID="144bfa2c-73fd-4665-bf1f-b740648b6b59" TYPE="ext4" PARTLABEL="old-agent-id-1" PARTUUID="024fe371-91a3-4835-af4e-c9182702cbb6"
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
							SizeInBytes: 8589934592, // (16GiB)
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

					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(15))

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/edx", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"blkid"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "-a", "/dev/edx1"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "-a", "/dev/edx2"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "/dev/edx", "rm", "1"}))
					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "/dev/edx", "rm", "2"}))
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

				err := partitioner.Partition("/dev/edx", partitions)
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
				fakeCmdRunner.AddCmdResult(
					"blkid",
					fakesys.FakeCmdResult{
						Stdout: "Some weird error", ExitStatus: 1, Error: errors.New("Some weird error")},
				)
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/edx unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/edx:221190815744B:xvd:512:512:gpt:Xen Virtual Block Device;
`},
				)
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

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(err.Error()).To(ContainSubstring("Removing existing partitions of `/dev/edx'"))
			})
		})
	})

	Describe("GetDeviceSizeInBytes", func() {
		It("returns number in bytes (stripping newline) from lsblk", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/edx",
				fakesys.FakeCmdResult{Stdout: "123\n"},
			)

			num, err := partitioner.GetDeviceSizeInBytes("/dev/edx")
			Expect(err).ToNot(HaveOccurred())
			Expect(num).To(Equal(uint64(123)))
		})

		It("returns error if lsblk fails", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/edx",
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)

			_, err := partitioner.GetDeviceSizeInBytes("/dev/edx")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-err"))
		})
	})
})
