package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("rootDevicePartitioner", func() {
	var (
		fakeCmdRunner *fakesys.FakeCmdRunner
		partitioner   Partitioner
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fakeCmdRunner = fakesys.NewFakeCmdRunner()
		partitioner = NewRootDevicePartitioner(logger, fakeCmdRunner, 1)
	})

	Describe("Partition", func() {
		Context("when the desired partitions do not exist", func() {
			BeforeEach(func() {
				// 20GiB device, ~3GiB partition 0, 18403868671B remaining
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/vda:21474836480B:virtblk:512:512:msdos:Virtio Block Device;
1:32256B:3071000063B:3070967808B:ext4::;
`,
					},
				)
			})

			It("creates partitions (aligned to 1MiB) using parted", func() {
				partitions := []Partition{
					{SizeInBytes: 8589934592}, // swap (8GiB)
					{SizeInBytes: 8589934592}, // ephemeral (8GiB)
				}

				// Calculating "aligned" partition start/end/size
				// (3071000063 + 1) % 1048576 = 769536
				// (3071000063 + 1) + 1048576 - 769536 = 3071279104 (aligned start)
				// 3071279104 + 8589934592 - 1 = 11661213695 (desired end)
				// swap start=3071279104, end=11661213695, size=8589934592
				// (11661213695 + 1) % 1048576 = 0
				// (11661213695 + 1) = 11661213696 (aligned start)
				// 11661213696 + 8589934592 - 1 = 20251148287 (desired end)
				// 20251148287 > 21474836480 = false (smaller than disk)
				// swap start=11661213696, end=20251148287, size=8589934592

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "3071279104", "11661213695"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "11661213696", "20251148287"}))
			})

			It("truncates the last partition if it is larger than remaining disk space", func() {
				partitions := []Partition{
					{SizeInBytes: 8589934592}, // swap (8GiB)
					{SizeInBytes: 9813934079}, // ephemeral ("remaining" that exceeds disk size when aligned)
				}

				// Calculating "aligned" partition start/end/size
				// (3071000063 + 1) % 1048576 = 769536
				// (3071000063 + 1) + 1048576 - 769536 = 3071279104 (aligned start)
				// 3071279104 + 8589934592 - 1 = 11661213695 (desired end)
				// 11661213695 - 3071279104 + 1 = 8589934592 (final size)
				// swap start=3071279104, end=11661213695, size=8589934592
				// (11661213695 + 1) % 1048576 = 1048575
				// (11661213695 + 1) + 1048575  = 11662262271 (aligned start)
				// 11661213696 + 9813934079 - 1 = 21475147774 (desired end)
				// 21475147774 > 21474836480 = true (bigger than disk)
				// 21474836480 - 1 = 21474836479 (end of disk)
				// 21474836479 - 11661213696 + 1 = 9813622784 (final size from aligned start to end of disk)
				// swap start=11661213696, end=21474836479, size=9813622784

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "3071279104", "11661213695"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "11661213696", "21474836479"}))
			})

			Context("when partitioning fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda unit B mkpart primary 3071279104 11661213695",
						fakesys.FakeCmdResult{Error: errors.New("fake-parted-error")},
					)
				})

				It("returns error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // swap (8GiB)
						{SizeInBytes: 9813934079}, // ephemeral (remaining)
					}

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Partitioning disk `/dev/sda'"))
					Expect(err.Error()).To(ContainSubstring("fake-parted-error"))
				})
			})
		})

		Context("when the desired partitions do not exist with EFI partition", func() {
			BeforeEach(func() {
				// 20GiB device, ~50Mib partition 0, ~5GiB partition 1, 18403868671B remaining
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/vda:21474836480B:virtblk:512:512:msdos:Virtio Block Device;
1:1048576B:51380223B:50331648B:fat16::lba;
2:52428800B:5368709119B:5316280320B:ext4::;
`,
					},
				)
			})

			It("creates partitions (aligned to 1MiB) using parted", func() {
				partitions := []Partition{
					{SizeInBytes: 4294967296}, // swap (4GiB)
					{SizeInBytes: 4294967296}, // ephemeral (4GiB)
				}

				// Calculating "aligned" partition start/end/size
				// (5368709119 + 1) % 1048576 = 0
				// (5368709119 + 1) = 5368709120 (aligned start)
				// 5368709120 + 4294967296 - 1 = 9663676415 (desired end)
				// swap start=5369757696, end=9663676415, size=4294967296
				// (9663676415 + 1) % 1048576 = 0
				// (9663676415 + 1) = 9663676416 (aligned start)
				// 9663676416 + 4294967296 - 1 = 13958643711 (desired end)
				// 13958643711 > 21474836480 = false (smaller than disk)
				// swap start=9663676416, end=13958643711, size=4294967296

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "5368709120", "9663676415"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "9663676416", "13958643711"}))
			})

			It("truncates the last partition if it is larger than remaining disk space", func() {
				partitions := []Partition{
					{SizeInBytes: 8589934592}, // swap (8GiB)
					{SizeInBytes: 9813934079}, // ephemeral ("remaining" that exceeds disk size when aligned)
				}

				// Calculating "aligned" partition start/end/size
				// (5368709119 + 1) % 1048576 = 0
				// (5368709119 + 1) = 5368709120 (aligned start)
				// 5368709120 + 8589934592 - 1 = 13958643711 (desired end)
				// 11661213695 - 3071279104 + 1 = 8589934592 (final size)
				// swap start=5368709120, end=13958643711, size=8589934592
				// (13958643711 + 1) % 1048576 = 0
				// (13958643711 + 1) = 13958643712 (aligned start)
				// 13958643712 + 9813934079 - 1 = 23772577790 (desired end)
				// 23772577790 > 21474836480 = true (bigger than disk)
				// 21474836480 - 1 = 21474836479 (end of disk)
				// 21474836479 - 13958643712 + 1 = 7516192768 (final size from aligned start to end of disk)
				// swap start=13958643712, end=21474836479, size=7516192768

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "5368709120", "13958643711"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "13958643712", "21474836479"}))
			})

			Context("when partitioning fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda unit B mkpart primary 5368709120 13958643711",
						fakesys.FakeCmdResult{Error: errors.New("fake-parted-error")},
					)
				})

				It("returns error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // swap (8GiB)
						{SizeInBytes: 9813934079}, // ephemeral (remaining)
					}

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Partitioning disk `/dev/sda'"))
					Expect(err.Error()).To(ContainSubstring("fake-parted-error"))
				})
			})
		})

		Context("when the desired partitions do not exist and there are 2 existing partitions", func() {
			BeforeEach(func() {
				// 20GiB device, ~3GiB partition 0, 18403868671B remaining
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:21474836480B:virtblk:512:512:msdos:Virtio Block Device;
1:0:0B:0B:::boot, prep;
2:32256B:3071000063B:3070967808B:ext4::;
`,
					},
				)
			})

			It("creates partitions (aligned to 1MiB) using parted", func() {
				partitions := []Partition{
					{SizeInBytes: 8589934592}, // swap (8GiB)
					{SizeInBytes: 8589934592}, // ephemeral (8GiB)
				}

				// Calculating "aligned" partition start/end/size
				// (3071000063 + 1) % 1048576 = 769536
				// (3071000063 + 1) + 1048576 - 769536 = 3071279104 (aligned start)
				// 3071279104 + 8589934592 - 1 = 11661213695 (desired end)
				// swap start=3071279104, end=11661213695, size=8589934592
				// (11661213695 + 1) % 1048576 = 0
				// (11661213695 + 1) = 11661213696 (aligned start)
				// 11661213696 + 8589934592 - 1 = 20251148287 (desired end)
				// 20251148287 > 21474836480 = false (smaller than disk)
				// swap start=11661213696, end=20251148287, size=8589934592

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "3071279104", "11661213695"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "11661213696", "20251148287"}))
			})

			It("truncates the last partition if it is larger than remaining disk space", func() {
				partitions := []Partition{
					{SizeInBytes: 8589934592}, // swap (8GiB)
					{SizeInBytes: 9813934079}, // ephemeral ("remaining" that exceeds disk size when aligned)
				}

				// Calculating "aligned" partition start/end/size
				// (3071000063 + 1) % 1048576 = 769536
				// (3071000063 + 1) + 1048576 - 769536 = 3071279104 (aligned start)
				// 3071279104 + 8589934592 - 1 = 11661213695 (desired end)
				// 11661213695 - 3071279104 + 1 = 8589934592 (final size)
				// swap start=3071279104, end=11661213695, size=8589934592
				// (11661213695 + 1) % 1048576 = 0
				// (11661213695 + 1) = 11661213696 (aligned start)
				// 11661213696 + 9813934079 - 1 = 21475147774 (desired end)
				// 21475147774 > 21474836480 = true (bigger than disk)
				// 21474836480 - 1 = 21474836479 (end of disk)
				// 21474836479 - 11661213696 + 1 = 9813622784 (final size from aligned start to end of disk)
				// swap start=11661213696, end=21474836479, size=9813622784

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())

				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(3))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "3071279104", "11661213695"}))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "primary", "11661213696", "21474836479"}))
			})

			Context("when partitioning fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda unit B mkpart primary 3071279104 11661213695",
						fakesys.FakeCmdResult{Error: errors.New("fake-parted-error")},
					)
				})

				It("returns error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // swap (8GiB)
						{SizeInBytes: 9813934079}, // ephemeral (remaining)
					}

					err := partitioner.Partition("/dev/sda", partitions)
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
				partitions := []Partition{{SizeInBytes: 32}}

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Getting existing partitions of `/dev/sda'"))
				Expect(err.Error()).To(ContainSubstring("Running parted print on `/dev/sda'"))
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
1:1B:32B:32B:ext4::;
2:33B:64B:32B:ext4::;
`,
					},
				)
			})

			It("does not partition", func() {
				partitions := []Partition{{SizeInBytes: 32}}

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
			})
		})

		Context("when partitions already match with EFI partition", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
1:1B:16B:16B:fat16::;
2:17B:80B:64B:ext4::;
3:81B:114B:32B:ext4::;
`,
					},
				)
			})

			It("does not partition", func() {
				partitions := []Partition{{SizeInBytes: 32}}

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
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
				partitions := []Partition{
					{SizeInBytes: 32},
				}

				err := partitioner.Partition("/dev/sda", partitions)
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
				partitions := []Partition{
					{SizeInBytes: 32},
				}

				err := partitioner.Partition("/dev/sda", partitions)
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
1:1B:32B:32B:ext4::;
2:0.2B:65B:32B:ext4::;
`,
					},
				)
			})

			It("returns an error", func() {
				partitions := []Partition{
					{SizeInBytes: 32},
				}

				err := partitioner.Partition("/dev/sda", partitions)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Parsing existing partitions of `/dev/sda'"))
				Expect(len(fakeCmdRunner.RunCommands)).To(Equal(1))
				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
			})
		})
	})

	Describe("GetDeviceSizeInBytes", func() {
		Context("when getting disk partition information succeeds", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:128B:virtblk:512:512:msdos:Virtio Block Device;
1:15B:31B:17B:ext4::;
2:32B:54B:23B:ext4::;
`,
					},
				)
			})

			It("returns the size of the device", func() {
				size, err := partitioner.GetDeviceSizeInBytes("/dev/sda")
				Expect(err).ToNot(HaveOccurred())
				Expect(size).To(Equal(uint64(96)))
			})
		})

		Context("when getting disk partition with EFI boot disk information succeeds", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/sda unit B print",
					fakesys.FakeCmdResult{
						Stdout: `BYT;
/dev/sda:53687091200B:virtblk:512:512:msdos:Virtio Block Device;
1:1048576B:51380223B:50331648B:fat16::lba;
2:52428800B:5368709119B:5316280320B:ext4::;
`,
					},
				)
			})

			It("returns the size of the device", func() {
				size, err := partitioner.GetDeviceSizeInBytes("/dev/sda")
				Expect(err).ToNot(HaveOccurred())
				Expect(size).To(Equal(uint64(48318382080)))
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
				size, err := partitioner.GetDeviceSizeInBytes("/dev/sda")
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
				size, err := partitioner.GetDeviceSizeInBytes("/dev/sda")
				Expect(err).To(HaveOccurred())
				Expect(size).To(Equal(uint64(0)))
				Expect(err.Error()).To(ContainSubstring("Getting remaining size of `/dev/sda'"))
			})
		})
	})
})
