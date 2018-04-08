package disk_test

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/platform/disk"
	"github.com/cloudfoundry/bosh-agent/platform/disk/fakes"
	"github.com/cloudfoundry/bosh-utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PersistentDevicePartitioner", func() {
	var (
		partitioner       disk.Partitioner
		sfDiskPartitioner *fakes.FakePartitioner
		partedPartitioner *fakes.FakePartitioner
		diskUtil          *fakes.FakeDiskUtil

		devicePath string
		partitions []disk.Partition
	)

	BeforeEach(func() {
		devicePath = "/dev/jim"
		partitions = []disk.Partition{
			{
				SizeInBytes: 100,
				Type:        disk.PartitionTypeLinux,
			},
			{
				SizeInBytes: 101,
				Type:        disk.PartitionTypeSwap,
			},
		}

		logger := logger.NewLogger(logger.LevelNone)
		sfDiskPartitioner = fakes.NewFakePartitioner()
		partedPartitioner = fakes.NewFakePartitioner()
		diskUtil = fakes.NewFakeDiskUtil()

		partitioner = disk.NewPersistentDevicePartitioner(sfDiskPartitioner, partedPartitioner, diskUtil, logger)
	})

	Describe("Partition", func() {
		It("uses sfdisk to partition the persistent disk", func() {
			err := partitioner.Partition(devicePath, partitions)
			Expect(err).NotTo(HaveOccurred())

			Expect(diskUtil.GetBlockDeviceSizeDiskPath).To(Equal(devicePath))

			Expect(sfDiskPartitioner.PartitionCalled).To(BeTrue())
			Expect(sfDiskPartitioner.PartitionDevicePath).To(Equal(devicePath))
			Expect(sfDiskPartitioner.PartitionPartitions).To(Equal(partitions))
		})

		Context("when fetching the disk size fails", func() {
			BeforeEach(func() {
				diskUtil.GetBlockDeviceSizeError = errors.New("boom")
			})

			It("uses the sfdisk partitioner", func() {
				err := partitioner.Partition(devicePath, partitions)
				Expect(err).NotTo(HaveOccurred())

				Expect(sfDiskPartitioner.PartitionCalled).To(BeTrue())
				Expect(sfDiskPartitioner.PartitionDevicePath).To(Equal(devicePath))
				Expect(sfDiskPartitioner.PartitionPartitions).To(Equal(partitions))
			})
		})

		Context("when sfdisk fails to partition", func() {
			Context("because of a GPT error", func() {
				BeforeEach(func() {
					sfDiskPartitioner.PartitionErr = disk.ErrGPTPartitionEncountered
				})

				It("partitions with parted", func() {
					err := partitioner.Partition(devicePath, partitions)
					Expect(err).NotTo(HaveOccurred())

					Expect(sfDiskPartitioner.PartitionCalled).To(BeTrue())
					Expect(sfDiskPartitioner.PartitionDevicePath).To(Equal(devicePath))
					Expect(sfDiskPartitioner.PartitionPartitions).To(Equal(partitions))

					Expect(partedPartitioner.PartitionCalled).To(BeTrue())
					Expect(partedPartitioner.PartitionDevicePath).To(Equal(devicePath))
					Expect(partedPartitioner.PartitionPartitions).To(Equal(partitions))
				})
			})

			Context("because of an unexpected error", func() {
				BeforeEach(func() {
					sfDiskPartitioner.PartitionErr = errors.New("boom")
				})

				It("returns an error", func() {
					err := partitioner.Partition(devicePath, partitions)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the size of the disk to be partitioned is larger than 2 TB", func() {
			BeforeEach(func() {
				diskUtil.GetBlockDeviceSizeSize = disk.MaxFdiskPartitionSize + 1
			})

			It("uses the parted partitioner", func() {
				err := partitioner.Partition(devicePath, partitions)
				Expect(err).NotTo(HaveOccurred())

				Expect(sfDiskPartitioner.PartitionCalled).To(BeFalse())
				Expect(partedPartitioner.PartitionCalled).To(BeTrue())
				Expect(partedPartitioner.PartitionDevicePath).To(Equal(devicePath))
				Expect(partedPartitioner.PartitionPartitions).To(Equal(partitions))
			})
		})
	})

	Describe("GetDeviceSizeInBytes", func() {
		BeforeEach(func() {
			sfDiskPartitioner.GetDeviceSizeInBytesSizes = map[string]uint64{
				"/dev/jim":       10000,
				"/dev/who-cares": 20000,
			}
		})

		It("uses the sfdisk partitioner", func() {
			size, err := partitioner.GetDeviceSizeInBytes("/dev/jim")
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(BeEquivalentTo(10000))
		})

		Context("when sfdisk return an error", func() {
			BeforeEach(func() {
				sfDiskPartitioner.GetDeviceSizeInBytesErr = errors.New("nice try")
			})

			It("returns an error", func() {
				_, err := partitioner.GetDeviceSizeInBytes("/dev/jim")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
