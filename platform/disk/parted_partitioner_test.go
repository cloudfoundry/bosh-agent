package disk_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakeboshaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

const partitionNamePrefix = "bosh-partition"

func bytesOfMiB(sizeInMiB uint64) uint64 {
	return sizeInMiB * 1024 * 1024
}

func bytesOfGiB(sizeInGiB uint64) uint64 {
	return bytesOfMiB(sizeInGiB) * 1024
}

type PartedPartition struct {
	Index      int
	Start      uint64
	End        uint64
	Size       uint64
	Filesystem string
	Name       string
}

func buildPartedOutput(
	devicePath string, deviceSize uint64,
	transportType, partitionTableType, modelName string,
	partitions []PartedPartition) (partedOutput string) {
	partedOutput = "BYT;\n"
	// Disk info format:
	// "path":"size":"transport-type":"logical-sector-size":"physical-sector-size":"partition-table-type":"model-name";
	// See: https://alioth-lists.debian.net/pipermail/parted-devel/2006-December/000573.html
	partedOutput += fmt.Sprintf(
		"%s:%dB:%s:512:512:%s:%s;\n",
		devicePath,
		deviceSize,
		transportType,
		partitionTableType,
		modelName,
	)
	for _, partition := range partitions {
		// Partition info format:
		// "number":"begin":"end":"size":"filesystem-type":"partition-name":"flags-set";
		partedOutput += fmt.Sprintf(
			"%d:%dB:%dB:%dB:%s:%s:;\n",
			partition.Index,
			partition.Start,
			partition.End,
			partition.Size,
			partition.Filesystem,
			partition.Name,
		)
	}
	return
}

var _ = Describe("PartedPartitioner", func() {
	var (
		fakeCmdRunner *fakesys.FakeCmdRunner
		partitioner   Partitioner
		fakeclock     *fakeboshaction.FakeClock
		logger        boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
		fakeCmdRunner = fakesys.NewFakeCmdRunner()
		fakeclock = &fakeboshaction.FakeClock{}
		partitioner = NewPartedPartitioner(logger, fakeCmdRunner, fakeclock)
	})

	Describe("Partition", func() {
		Context("when the desired partitions do not exist", func() {
			Context("when there is no partition table", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout:     "Error: /dev/sda: unrecognised disk label",
							ExitStatus: 1,
							Error:      errors.New("Error: /dev/sda: unrecognised disk label"),
						})
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{}),
						})
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda mklabel gpt",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 0})
					fakeCmdRunner.AddCmdResult(
						"udevadm settle",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 0, Sticky: true})
				})

				It("makes a gpt label and then creates partitions using parted", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					// Calculating "aligned" partition start/end/size
					// (512 + 1) % 1048576 = 513
					// (512 + 1) + 1048576 - 513 = 1048576 (aligned start)
					// 1048576 + 8589934592 = 8590983168
					// 8590983168 % 1048576 = 0
					// 8590983168 - 0 - 1 = 8590983167 (desired end)
					// first start=1048576, end=8590983167, size=8589934592

					// (8590983167 + 1) % 1048576 = 0
					// (8590983167 + 1) = 8590983168 (aligned start)
					// 8590983168 + 8589934592 = 17180917760 (desired end)
					// 17180917760 % 1048576 = 0
					// 17180917760 - 0 - 1 = 17180917759
					// second start=11661213696, end=17180917759, size=8589934592
					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"parted", "-s", "/dev/sda", "mklabel", "gpt"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "8590983167"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-1", "8590983168", "17180917759"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when there is a loop partition table", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/vdb", bytesOfGiB(20), "virtblk", "loop", "Virtio Block Device",
								[]PartedPartition{}),
						})
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{}),
						})

					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda mklabel gpt",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 0})
					fakeCmdRunner.AddCmdResult(
						"udevadm settle",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 0, Sticky: true})
				})

				It("ignores the loop partition table and assumes an empty disk", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					// Calculating "aligned" partition start/end/size
					// (512 + 1) % 1048576 = 513
					// (512 + 1) + 1048576 - 513 = 1048576 (aligned start)
					// 1048576 + 8589934592 = 8590983168
					// 8590983168 % 1048576 = 0
					// 8590983168 - 0 - 1 = 8590983167 (desired end)
					// first start=1048576, end=8590983167, size=8589934592

					// (8590983167 + 1) % 1048576 = 0
					// (8590983167 + 1) = 8590983168 (aligned start)
					// 8590983168 + 8589934592 = 17180917760 (desired end)
					// 17180917760 % 1048576 = 0
					// 17180917760 - 0 - 1 = 17180917759
					// second start=11661213696, end=17180917759, size=8589934592

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"parted", "-s", "/dev/sda", "mklabel", "gpt"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "8590983167"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-1", "8590983168", "17180917759"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when there are no partitions", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{}),
						})
				})

				It("creates partitions using parted starting at the 1048576 byte", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					// Calculating "aligned" partition start/end/size
					// (512 + 1) % 1048576 = 513
					// (512 + 1) + 1048576 - 513 = 1048576 (aligned start)
					// 1048576 + 8589934592 = 8590983168
					// 8590983168 % 1048576 = 0
					// 8590983168 - 0 - 1 = 8590983167 (desired end)
					// first start=1048576, end=8590983167, size=8589934592

					// (8590983167 + 1) % 1048576 = 0
					// (8590983167 + 1) = 8590983168 (aligned start)
					// 8590983168 + 8589934592 = 17180917760 (desired end)
					// 17180917760 % 1048576 = 0
					// 17180917760 - 0 - 1 = 17180917759
					// second start=11661213696, end=17180917759, size=8589934592

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "8590983167"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-1", "8590983168", "17180917759"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
					}))
				})

			})

			Context("when there are existing partitions", func() {

				Context("and none of the partitions were created by BOSH", func() {
					BeforeEach(func() {
						fakeCmdRunner.AddCmdResult(
							"parted -m /dev/sda unit B print",
							fakesys.FakeCmdResult{
								Stdout: buildPartedOutput(
									"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
									[]PartedPartition{
										{Index: 1,
											Start: 512, End: 2048576, Size: 199680,
											Filesystem: "ext4", Name: "primary"},
									}),
							})
					})

					It("creates partitions using parted overwriting the existing partitions", func() {
						partitions := []Partition{
							{SizeInBytes: 8589934592}, // (8GiB)
							{SizeInBytes: 8589934592}, // (8GiB)
						}

						// Calculating "aligned" partition start/end/size
						// (513) % 1048576 = 513
						// (513) + 1048576 - 513 = 1048576 (aligned start)
						// 1048576 + 8589934592 = 8590983168
						// 8590983168 % 1048576 = 0
						// 8590983168 - 0 - 1 = 8590983167 (desired end)
						// first start=1048576, end=8590983167, size=8589934592

						// (8590983167 + 1) % 1048576 = 0
						// (8590983167 + 1) = 8590983168 (aligned start)
						// 8590983168 + 8589934592 = 17180917760 (desired end)
						// 17180917760 % 1048576 = 0
						// 17180917760 - 0 - 1 = 17180917759
						// second start=8590983168, end=17180917759, size=8589934592

						err := partitioner.Partition("/dev/sda", partitions)
						Expect(err).ToNot(HaveOccurred())

						Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
							{"partprobe", "/dev/sda"},
							{"parted", "-m", "/dev/sda", "unit", "B", "print"},
							{"udevadm", "settle"},
							{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "8590983167"},
							{"partprobe", "/dev/sda"},
							{"udevadm", "settle"},
							{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-1", "8590983168", "17180917759"},
							{"partprobe", "/dev/sda"},
							{"udevadm", "settle"},
						}))
					})
				})

				Context("and a partition was created by BOSH", func() {
					BeforeEach(func() {
						fakeCmdRunner.AddCmdResult(
							"parted -m /dev/sda unit B print",
							fakesys.FakeCmdResult{
								Stdout: buildPartedOutput(
									"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
									[]PartedPartition{
										{Index: 1,
											Start: 512, End: 2048576, Size: 199680,
											Filesystem: "ext4", Name: "bosh-partition-0"},
									}),
							})
					})

					It("does NOT partition the disk, and returns an error", func() {
						partitions := []Partition{
							{SizeInBytes: 8589934592}, // (8GiB)
							{SizeInBytes: 8589934592}, // (8GiB)
						}

						// Calculating "aligned" partition start/end/size
						// (513) % 1048576 = 513
						// (513) + 1048576 - 513 = 1048576 (aligned start)
						// 1048576 + 8589934592 = 8590983168
						// 8590983168 % 1048576 = 0
						// 8590983168 - 0 - 1 = 8590983167 (desired end)
						// first start=1048576, end=8590983167, size=8589934592

						// (8590983167 + 1) % 1048576 = 0
						// (8590983167 + 1) = 8590983168 (aligned start)
						// 8590983168 + 8589934592 = 17180917760 (desired end)
						// 17180917760 % 1048576 = 0
						// 17180917760 - 0 - 1 = 17180917759
						// second start=8590983168, end=17180917759, size=8589934592

						err := partitioner.Partition("/dev/sda", partitions)
						Expect(err.Error()).To(Equal("'/dev/sda' contains a partition created by bosh. No partitioning is allowed."))

						Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
							{"partprobe", "/dev/sda"},
							{"parted", "-m", "/dev/sda", "unit", "B", "print"},
							{"udevadm", "settle"},
						}))
					})
				})

			})

			Context("when the type does not match", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sdf unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(2930), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: bytesOfMiB(1), End: 3146062496255, Size: 3146062495744,
										Filesystem: "Golden Bow", Name: "primary"},
								}),
						})
				})

				It("replaces the partition", func() {
					partitions := []Partition{
						{Type: PartitionTypeLinux},
					}

					// Calculating "aligned" partition start/end/size
					// (513) % 1048576 = 513
					// (513) + 1048576 - 513 = 1048576 (aligned start)
					// 1048576 + 3146063544320 = 3146064592896
					// min(3146064592896, 3146063544320 - 1) = 3146063544319
					// 3146063544319 % 1048576 = 1048575
					// 3146063544319 - 1048575 - 1 = 3146062495743 (desired end)
					// first start=1048576, end=3146062495743, size=3146062495743

					err := partitioner.Partition("/dev/sdf", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sdf"},
						{"parted", "-m", "/dev/sdf", "unit", "B", "print"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sdf", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "3146062495743"},
						{"partprobe", "/dev/sdf"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when the partition is not yet formatted", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sdf unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(2930), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: bytesOfMiB(1), End: 3146062496255, Size: 3146062495744,
										Filesystem: "", Name: "primary"},
								}),
						})
				})

				It("repartitions", func() {
					partitions := []Partition{
						{Type: PartitionTypeLinux},
					}

					// Calculating "aligned" partition start/end/size
					// (513) % 1048576 = 513
					// (513) + 1048576 - 513 = 1048576 (aligned start)
					// 1048576 + 3146063544320 = 3146064592896
					// min(3146064592896, 3146063544320 - 1) = 3146063544319
					// 3146063544319 % 1048576 = 1048575
					// 3146063544319 - 1048575 - 1 = 3146062495743 (desired end)
					// first start=1048576, end=3146062495743, size=3146062495743

					err := partitioner.Partition("/dev/sdf", partitions)
					Expect(err).ToNot(HaveOccurred())
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sdf"},
						{"parted", "-m", "/dev/sdf", "unit", "B", "print"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sdf", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "3146062495743"},
						{"partprobe", "/dev/sdf"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when the required partition over-flows the device", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: 512, End: 2048576, Size: 199680,
										Filesystem: "ext4", Name: ""},
								}),
						})
				})

				It("creates partitions using parted but truncates the partition", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592},   // (8GiB)
						{SizeInBytes: 221190815744}, // (197GiB)
					}

					// Calculating "aligned" partition start/end/size
					// (513) % 1048576 = 513
					// (513) + 1048576 - 513 = 1048576 (aligned start)
					// 1048576 + 8589934592 = 8590983168
					// 8590983168 % 1048576 = 0
					// 8590983168 - 0 - 1 = 8590983167 (desired end)
					// first start=1048576, end=8590983167, size=8589934592

					// (8590983167 + 1) % 1048576 = 0
					// (8590983167 + 1) = 8590983168 (aligned start)
					// 8590983168 + 221190815744 = 229781798912 (desired end)
					// min(229781798912, 221190815744 - 1) = 221190815743
					// 221190815743 % 1048576 = 1048575
					// 221190815743 - 1048575 - 1 = 221189767167
					// second start=8590983168, end=221189767167, size=212599832575

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-0", "1048576", "8590983167"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
						{"parted", "-s", "/dev/sda", "unit", "B", "mkpart", "bosh-partition-1", "8590983168", "221189767167"},
						{"partprobe", "/dev/sda"},
						{"udevadm", "settle"},
					}))
				})
			})
		})

		Context("when the existing partitions match desired partitions", func() {
			Context("when the partitions match exactly", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: 512, End: 8589935104, Size: 8589934592,
										Filesystem: "ext4", Name: ""},
									{Index: 2,
										Start: 8589935105, End: 17179869697, Size: 8589934592,
										Filesystem: "ext4", Name: ""},
								}),
						})
				})

				It("checks the existing partitions and does nothing", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592, Type: PartitionTypeLinux}, // (8GiB)
						{SizeInBytes: 8589934592, Type: PartitionTypeLinux}, // (8GiB)
					}
					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when the partitions are within delta", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: 512, End: 8589935104, Size: 8558963072,
										Filesystem: "ext4", Name: ""},
									{Index: 2,
										Start: 8589935105, End: 17179869697, Size: 8568963072,
										Filesystem: "ext4", Name: ""},
								}),
						})
				})

				It("checks the existing partitions and does nothing", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592, Type: PartitionTypeLinux}, // (8GiB)
						{SizeInBytes: 8589934592, Type: PartitionTypeLinux}, // (8GiB)
					}
					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when we have extra partitions", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(206), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: 512, End: 8589935104, Size: 8589934592,
										Filesystem: "ext4", Name: ""},
									{Index: 2,
										Start: 8589935105, End: 17179869697, Size: 8589934592,
										Filesystem: "ext4", Name: ""},
								}),
						})
				})

				It("checks the existing partitions and does nothing", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592, Type: PartitionTypeLinux}, // (8GiB)
					}
					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sda", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when there is an existing partition within the expected size and type", func() {
				for _, fsFormat := range []string{"ext4", "xfs"} {
					Context(fmt.Sprintf("with %s filesystem", fsFormat), func() {
						BeforeEach(func() {
							fakeCmdRunner.AddCmdResult(
								"parted -m /dev/sdf unit B print",
								fakesys.FakeCmdResult{
									Stdout: buildPartedOutput(
										"/dev/xvdf", bytesOfGiB(2930), "xvd", "gpt", "Xen Virtual Block Device",
										[]PartedPartition{
											{Index: 1,
												Start: 1048576, End: 3146062496255, Size: 3146062495744,
												Filesystem: fsFormat, Name: "primary"},
										}),
								})
						})

						It("reuses the existing partition", func() {
							partitions := []Partition{
								{Type: PartitionTypeLinux},
							}

							err := partitioner.Partition("/dev/sdf", partitions)
							Expect(err).ToNot(HaveOccurred())

							Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
								{"partprobe", "/dev/sdf"},
								{"parted", "-m", "/dev/sdf", "unit", "B", "print"},
								{"udevadm", "settle"},
							}))
						})
					})
				}
			})

			Context("when a swap partition is used", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sdf unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/xvdf", bytesOfGiB(2930), "xvd", "gpt", "Xen Virtual Block Device",
								[]PartedPartition{
									{Index: 1,
										Start: 1048576, End: 3146062496255, Size: 3146062495744,
										Filesystem: "linux-swap(v1)", Name: "primary"},
								}),
						})
				})

				It("reuses the existing partition", func() {
					partitions := []Partition{
						{Type: PartitionTypeSwap},
					}

					err := partitioner.Partition("/dev/sdf", partitions)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"parted", "-m", "/dev/sdf", "unit", "B", "print"}))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sdf"},
						{"parted", "-m", "/dev/sdf", "unit", "B", "print"},
						{"udevadm", "settle"},
					}))
				})
			})
		})

		Context("when getting existing partitions returns an error", func() {
			Context("when re-reading partition table fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"partprobe /dev/sda",
						fakesys.FakeCmdResult{
							Stdout: "Some weird error", ExitStatus: 1,
							Error: errors.New("Some weird error"),
						})
				})

				It("throw an error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())

					Expect(err.Error()).To(ContainSubstring("Re-reading partition table for `/dev/sda': Some weird error"))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
					}))
				})
			})

			Context("when the first call to parted print fails", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: "Some weird error", ExitStatus: 1,
							Error: errors.New("Some weird error"),
						})
				})

				It("throw an error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())

					Expect(err.Error()).To(ContainSubstring("Getting existing partitions of `/dev/sda': Running parted print: Some weird error"))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when parted fails to make device label", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stderr: "Error: /dev/sda: unrecognised disk label", ExitStatus: 0},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda mklabel gpt",
						fakesys.FakeCmdResult{Stdout: "Some weird error", ExitStatus: 1, Error: errors.New("Some weird error")})
				})

				It("throw an error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())

					Expect(err.Error()).To(ContainSubstring("Getting existing partitions of `/dev/sda': Running parted print: Parted making label: Some weird error"))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"parted", "-s", "/dev/sda", "mklabel", "gpt"},
						{"udevadm", "settle"},
					}))
				})
			})

			Context("when parted makes a label but fails print the second time", func() {
				BeforeEach(func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stderr: "Error: /dev/sda: unrecognised disk label", ExitStatus: 0},
					)
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/sda unit B print",
						fakesys.FakeCmdResult{
							Stdout: `Some weird error`, Error: errors.New("Some weird error")})
					fakeCmdRunner.AddCmdResult(
						"parted -s /dev/sda mklabel gpt",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 0})
				})

				It("throw an error", func() {
					partitions := []Partition{
						{SizeInBytes: 8589934592}, // (8GiB)
						{SizeInBytes: 8589934592}, // (8GiB)
					}

					err := partitioner.Partition("/dev/sda", partitions)
					Expect(err).To(HaveOccurred())

					Expect(err.Error()).To(ContainSubstring("Getting existing partitions of `/dev/sda': Running parted print: Some weird error"))
					Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
						{"partprobe", "/dev/sda"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"parted", "-s", "/dev/sda", "mklabel", "gpt"},
						{"parted", "-m", "/dev/sda", "unit", "B", "print"},
						{"udevadm", "settle"},
					}))
				})
			})
		})
	})

	Describe("GetDeviceSizeInBytes", func() {
		It("returns error if lsblk fails", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/path",
				fakesys.FakeCmdResult{Error: errors.New("fake-err")},
			)

			_, err := partitioner.GetDeviceSizeInBytes("/dev/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-err"))
		})

		It("returns error if lsblk doesnt return number", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/path",
				fakesys.FakeCmdResult{Stdout: "not-number"},
			)

			_, err := partitioner.GetDeviceSizeInBytes("/dev/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Converting block device size"))
			Expect(err.Error()).To(ContainSubstring(`parsing "not-number"`))
		})

		It("returns number in bytes (stripping newline) from lsblk", func() {
			fakeCmdRunner.AddCmdResult(
				"lsblk --nodeps -nb -o SIZE /dev/path",
				fakesys.FakeCmdResult{Stdout: "123\n"},
			)

			num, err := partitioner.GetDeviceSizeInBytes("/dev/path")
			Expect(err).ToNot(HaveOccurred())
			Expect(num).To(Equal(uint64(123)))
		})
	})

	Describe("RemovePartitions", func() {
		Context("when there are existing partitions", func() {
			var existingPartitions []ExistingPartition

			BeforeEach(func() {
				existingPartitions = []ExistingPartition{
					{
						Index:        1,
						SizeInBytes:  uint64(4130340864),
						StartInBytes: uint64(1048576),
						EndInBytes:   uint64(413138943),
						Type:         PartitionTypeSwap,
						Name:         "bosh-partition-0",
					},
					{
						Index:        2,
						SizeInBytes:  uint64(103241744384),
						StartInBytes: uint64(4131389440),
						EndInBytes:   uint64(107373133823),
						Type:         PartitionTypeLinux,
						Name:         "bosh-partition-1",
					},
				}
			})

			It("removes partitions", func() {
				fakeCmdRunner.AddCmdResult(
					"wipefs --force -a /dev/sda",
					fakesys.FakeCmdResult{Stdout: "", ExitStatus: 0},
				)

				err := partitioner.RemovePartitions(existingPartitions, "/dev/sda")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeCmdRunner.RunCommands).To(Equal([][]string{
					{"wipefs", "--force", "-a", "/dev/sda"},
				}))
			})

			It("failed to remove partitions when removing device path error", func() {
				for i := 0; i < 20; i++ {
					fakeCmdRunner.AddCmdResult(
						"wipefs --force -a /dev/sda",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 2, Error: errors.New("fake-cmd-error")},
					)
				}

				err := partitioner.RemovePartitions(existingPartitions, "/dev/sda")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Removing device path"))

				Expect(fakeCmdRunner.RunCommands).To(ContainElement([]string{"wipefs", "--force", "-a", "/dev/sda"}))
			})
		})
	})

	Describe("SinglePartitionNeedsResize", func() {
		Context("when listing partitions fails", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult("parted -m /dev/nvme2n1 unit B print",
					fakesys.FakeCmdResult{ExitStatus: 1, Error: errors.New("No GPT found")})
			})

			It("returns an error", func() {
				_, err := partitioner.SinglePartitionNeedsResize("/dev/nvme2n1", PartitionTypeLinux)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Failed to get existing partitions"))
				Expect(err.Error()).To(ContainSubstring("No GPT found"))
			})
		})

		Context("when persistent disk has no partition", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/nvme2n1 unit B print",
					fakesys.FakeCmdResult{
						Stdout: buildPartedOutput(
							"/dev/nvme2n1", bytesOfGiB(4), "nvme", "gpt", "Amazon Elastic Block Store",
							[]PartedPartition{}),
					})
			})

			It("tells no re-partitioning is supposed to happen", func() {
				needsResize, err := partitioner.SinglePartitionNeedsResize("/dev/nvme2n1", PartitionTypeLinux)

				Expect(err).ToNot(HaveOccurred())
				Expect(needsResize).To(BeFalse())
			})
		})

		Context("when persistent disk has many existing partitions", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/nvme2n1 unit B print",
					fakesys.FakeCmdResult{
						Stdout: buildPartedOutput(
							"/dev/nvme2n1", bytesOfGiB(32), "nvme", "gpt", "Amazon Elastic Block Store",
							[]PartedPartition{
								{Index: 1,
									Start: bytesOfMiB(1), End: 17180917759, Size: 17179869184,
									Filesystem: "linux-swap(v1)", Name: "bosh-partition-0"},
								{Index: 2,
									Start: 17180917760, End: 34358689791, Size: 17177772032,
									Filesystem: "ext4", Name: "bosh-partition-1"},
							}),
					})
			})

			It("returns an error", func() {
				_, err := partitioner.SinglePartitionNeedsResize("/dev/nvme2n1", PartitionTypeLinux)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Persistent disks with many partitions are not supported"))
			})
		})

		Context("when persistent disk has one existing partition of swap type", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"parted -m /dev/nvme2n1 unit B print",
					fakesys.FakeCmdResult{
						Stdout: buildPartedOutput(
							"/dev/nvme2n1", bytesOfGiB(32), "nvme", "gpt", "Amazon Elastic Block Store",
							[]PartedPartition{
								{Index: 1,
									Start: bytesOfMiB(1), End: 2146435071, Size: 2145386496,
									Filesystem: "linux-swap(v1)", Name: "bosh-partition-0"},
							}),
					})
			})

			It("tells no re-partitioning is supposed to happen", func() {
				needsResize, err := partitioner.SinglePartitionNeedsResize("/dev/nvme2n1", PartitionTypeLinux)

				Expect(err).ToNot(HaveOccurred())
				Expect(needsResize).To(BeFalse())
			})
		})

		Context("when persistent disk has an existing partition of Linux type", func() {
			var (
				deviceSizeInBytes uint64
				setupPartedOutput func()
			)

			BeforeEach(func() {
				setupPartedOutput = func() {
					fakeCmdRunner.AddCmdResult(
						"parted -m /dev/nvme2n1 unit B print",
						fakesys.FakeCmdResult{
							Stdout: buildPartedOutput(
								"/dev/nvme2n1", deviceSizeInBytes, "nvme", "gpt", "Amazon Elastic Block Store",
								[]PartedPartition{
									{Index: 1,
										Start: bytesOfMiB(1), End: 2146435071, Size: 2145386496,
										Filesystem: "ext4", Name: "bosh-partition-0"},
								}),
						})
				}
			})

			Context("when device has slightly larger size due to geometry alignments", func() {
				BeforeEach(func() {
					deviceSizeInBytes = bytesOfGiB(2) + bytesOfMiB(20)
					setupPartedOutput()
				})

				It("tells the partition needs not being resized", func() {
					needsResize, err := partitioner.SinglePartitionNeedsResize("/dev/nvme2n1", PartitionTypeLinux)

					Expect(err).ToNot(HaveOccurred())
					Expect(needsResize).To(BeFalse())
				})
			})

			Context("when device has been grown in size", func() {
				BeforeEach(func() {
					deviceSizeInBytes = bytesOfGiB(4)
					setupPartedOutput()
				})

				It("tells the partition needs resizing", func() {
					needsResize, err := partitioner.SinglePartitionNeedsResize("/dev/nvme2n1", PartitionTypeLinux)

					Expect(err).ToNot(HaveOccurred())
					Expect(needsResize).To(BeTrue())
				})
			})
		})
	})

	Describe("ResizeSinglePartition", func() {
		BeforeEach(func() {
			fakeCmdRunner.AvailableCommands["growpart"] = true
			fakeCmdRunner.AvailableCommands["partx"] = true
		})

		Context("when growpart is missing", func() {
			BeforeEach(func() {
				fakeCmdRunner.AvailableCommands["growpart"] = false
			})

			It("returns an error", func() {
				err := partitioner.ResizeSinglePartition("/dev/nvme2n1")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("'growpart' is not installed"))
			})
		})

		Context("when partx is missing", func() {
			BeforeEach(func() {
				fakeCmdRunner.AvailableCommands["partx"] = false
			})

			It("returns an error", func() {
				err := partitioner.ResizeSinglePartition("/dev/nvme2n1")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("'partx' is not installed"))
			})
		})

		Context("when growing partition", func() {
			It("resizes paritions", func() {
				err := partitioner.ResizeSinglePartition("/dev/nvme2n1")

				Expect(err).ToNot(HaveOccurred())
				Expect(fakeCmdRunner.RunCommands).To(HaveLen(1))
				Expect(fakeCmdRunner.RunCommands[0]).To(Equal([]string{"growpart", "/dev/nvme2n1", "1", "--update", "auto"}))
			})
		})

		Context("when growpart temporarily fails once", func() {
			BeforeEach(func() {
				fakeCmdRunner.AddCmdResult(
					"growpart /dev/nvme2n1 1 --update auto",
					fakesys.FakeCmdResult{Stdout: "", ExitStatus: 1, Error: errors.New("growpart-failure")},
				)
			})

			It("reties and succeeds", func() {
				err := partitioner.ResizeSinglePartition("/dev/nvme2n1")

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCmdRunner.RunCommands[0]).To(Equal([]string{"growpart", "/dev/nvme2n1", "1", "--update", "auto"}))
				Expect(fakeCmdRunner.RunCommands[1]).To(Equal([]string{"growpart", "/dev/nvme2n1", "1", "--update", "auto"}))
			})
		})

		Context("when growpart fails constantly", func() {
			BeforeEach(func() {
				for i := 0; i < 20; i++ {
					fakeCmdRunner.AddCmdResult(
						"growpart /dev/nvme2n1 1 --update auto",
						fakesys.FakeCmdResult{Stdout: "", ExitStatus: 1, Error: errors.New("growpart-failure")},
					)
				}
			})

			It("returns an error", func() {
				err := partitioner.ResizeSinglePartition("/dev/nvme2n1")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("growpart-failure"))
			})
		})
	})
})
