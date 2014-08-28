package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
)

var _ = Describe("partedPartitioner", func() {
	var (
		fakeCmdRunner *fakesys.FakeCmdRunner
		partitioner   RootDevicePartitioner
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fakeCmdRunner = fakesys.NewFakeCmdRunner()
		partitioner = NewPartedPartitioner(logger, fakeCmdRunner)
	})

	Describe("CreatePartitions", func() {
		Context("when the desired partitions do not exist", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
1:1B:33B:32B:ext4::;
`,
					},
				)
			})

			It("creates partitions using parted", func() {
				partitions := []RootDevicePartition{
					{SizeInBytes: 32},
					{SizeInBytes: 64},
				}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "33", "65"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "65", "129"}))
			})

			Context("when partitioning fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda unit B mkpart primary 33 65",
						fakesys.FakeCmdResult{Error: errors.New("fake-parted-error")},
					)
				})

				It("returns error", func() {
					partitions := []RootDevicePartition{
						{SizeInBytes: 32},
					}

					err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Partitioning disk `/dev/sda'"))
					Expect(err.Error()).To(ContainSubstring("fake-parted-error"))
				})
			})
		})

		Context("when getting existing partitions fails", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{Error: errors.New("fake-parted-error")},
				)
			})

			It("returns error", func() {
				partitions := []RootDevicePartition{
					{SizeInBytes: 32},
				}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Partitioning disk `/dev/sda'"))
				Expect(err.Error()).To(ContainSubstring("Getting existing partitions of `/dev/sda'"))
				Expect(err.Error()).To(ContainSubstring("fake-parted-error"))
			})
		})

		Context("when partitions already match", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
1:1B:33B:32B:ext4::;
2:33B:65B:32B:ext4::;
`,
					},
				)
			})

			It("does not partition", func() {
				partitions := []RootDevicePartition{{SizeInBytes: 32}}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
			})
		})

		Context("when partition in the middle does not match", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
1:1B:33B:32B:ext4::;
2:33B:49B:16B:ext4::;
3:49B:81B:32B:ext4::;
4:81B:113B:32B:ext4::;
5:113B:121B:8B:ext4::;
`,
					},
				)
			})

			It("recreates partitions starting from middle partition", func() {
				partitions := []RootDevicePartition{
					{SizeInBytes: 16},
					{SizeInBytes: 16},
					{SizeInBytes: 32},
				}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(6))
				Expect(fakeCmdRunner.RunCommands[0]).To(Equal([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))

				Expect(fakeCmdRunner.RunCommands[1]).To(Equal([]string{"parted", "-s", "/dev/sda", "rm", "3"}))
				Expect(fakeCmdRunner.RunCommands[2]).To(Equal([]string{"parted", "-s", "/dev/sda", "rm", "4"}))
				Expect(fakeCmdRunner.RunCommands[3]).To(Equal([]string{"parted", "-s", "/dev/sda", "rm", "5"}))

				Expect(fakeCmdRunner.RunCommands[4]).To(Equal([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "49", "65"}))
				Expect(fakeCmdRunner.RunCommands[5]).To(Equal([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "65", "97"}))
			})

			Context("when removing existing partition fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda rm 3",
						fakesys.FakeCmdResult{Error: errors.New("fake-parted-error")},
					)
				})

				It("returns an error", func() {
					partitions := []RootDevicePartition{
						{SizeInBytes: 16},
						{SizeInBytes: 16},
						{SizeInBytes: 32},
					}

					err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Removing partition from `/dev/sda'"))
					Expect(err.Error()).To(ContainSubstring("Partitioning disk `/dev/sda'"))
					Expect(len(fakeCmdRunner.RunCommands)).To(Equal(2))
					Expect(fakeCmdRunner.RunCommands[0]).To(Equal([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))

					Expect(fakeCmdRunner.RunCommands[1]).To(Equal([]string{"parted", "-s", "/dev/sda", "rm", "3"}))
				})
			})
		})

		Context("when the first partition is missing", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
`,
					},
				)
			})

			It("returns an error", func() {
				partitions := []RootDevicePartition{
					{SizeInBytes: 32},
				}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Missing first partition on `/dev/sda'"))
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
			})
		})

		Context("when checking existing partitions does not return any result", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: "",
					},
				)
			})

			It("returns an error", func() {
				partitions := []RootDevicePartition{
					{SizeInBytes: 32},
				}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Parsing existing partitions of `/dev/sda'"))
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
			})
		})

		Context("when checking existing partitions does not return any result", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
1:1B:33B:32B:ext4::;
2:0.2B:65B:32B:ext4::;
`,
					},
				)
			})

			It("returns an error", func() {
				partitions := []RootDevicePartition{
					{SizeInBytes: 32},
				}

				err := partitioner.PartitionAfterFirstPartition("/dev/sda", partitions)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Parsing existing partitions of `/dev/sda'"))
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
			})
		})
	})

	Describe("GetRemainingSizeInMb", func() {
		Context("when getting disk partition information succeeds", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:33554560B:virtblk:512:512:msdos:Virtio Block Device;
1:1B:16777215B:16777216B:ext4::;
2:16777216B:33554432B:16777216B:ext4::;
`,
					},
				)
			})

			It("returns the size of the device", func() {
				size, err := partitioner.GetRemainingSizeInMb("/dev/sda")
				Expect(err).ToNot(HaveOccurred())
				Expect(size).To(Equal(uint64(16)))
			})
		})

		Context("when getting disk partition information fails", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Error: errors.New("fake-parted-error"),
					},
				)
			})

			It("returns an error", func() {
				size, err := partitioner.GetRemainingSizeInMb("/dev/sda")
				Expect(err).To(HaveOccurred())
				Expect(size).To(Equal(uint64(0)))
				Expect(err.Error()).To(ContainSubstring("fake-parted-error"))
			})
		})

		Context("when parsing parted result fails", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: ``,
					},
				)
			})

			It("returns an error", func() {
				size, err := partitioner.GetRemainingSizeInMb("/dev/sda")
				Expect(err).To(HaveOccurred())
				Expect(size).To(Equal(uint64(0)))
				Expect(err.Error()).To(ContainSubstring("Getting remaining size of `/dev/sda'"))
			})
		})
	})
})
