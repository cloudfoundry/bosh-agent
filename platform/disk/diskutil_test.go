package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshdevutil "github.com/cloudfoundry/bosh-agent/platform/deviceutil"
	fakedisk "github.com/cloudfoundry/bosh-agent/platform/disk/fakes"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/platform/disk"
)

var _ = Describe("Diskutil", func() {
	var (
		diskUtil boshdevutil.DeviceUtil
		mounter  *fakedisk.FakeMounter
		fs       *fakesys.FakeFileSystem
	)

	BeforeEach(func() {
		mounter = &fakedisk.FakeMounter{}
		fs = fakesys.NewFakeFileSystem()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		diskUtil = NewDiskUtil("fake-disk-path", mounter, fs, logger)
	})

	Describe("GetFileContents", func() {
		BeforeEach(func() {
			fs.TempDirDir = "fake-tempdir"
			fs.WriteFileString("fake-tempdir/fake-file-path", "fake-contents")
		})

		It("mounts disk path to temporary directory", func() {
			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).ToNot(HaveOccurred())

			Expect(mounter.MountPartitionPaths).To(ContainElement("fake-disk-path"))
			Expect(mounter.MountMountPoints).To(ContainElement("fake-tempdir"))
		})

		It("returns contents of file on a disk", func() {
			contents, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(contents)).To(Equal("fake-contents"))
		})

		It("unmount disk path", func() {
			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).ToNot(HaveOccurred())

			Expect(mounter.UnmountPartitionPathOrMountPoint).To(Equal("fake-disk-path"))
		})

		It("cleans up temporary directory after reading settings", func() {
			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).ToNot(HaveOccurred())

			Expect(fs.FileExists("fake-tempdir")).To(BeFalse())
		})

		It("returns error if it fails to create temporary mount directory", func() {
			fs.TempDirError = errors.New("fake-tempdir-error")

			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-tempdir-error"))
		})

		It("returns error if it fails to mount disk path", func() {
			mounter.MountErr = errors.New("fake-mount-error")

			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mount-error"))
		})

		It("returns an error if it fails to read the file", func() {
			fs.ReadFileError = errors.New("fake-read-error")
			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-read-error"))
		})

		It("returns error if it fails to unmount disk path", func() {
			mounter.UnmountErr = errors.New("fake-unmount-error")

			_, err := diskUtil.GetFileContents("fake-file-path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-unmount-error"))
		})
	})
})
