package disk_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	"errors"
)

var _ = Describe("Linux Formatter", func() {
	Describe("when using swap fs", func() {
		It("makes a swap disk", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemSwap)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mkswap", "/dev/xvda1"}))
		})

		It("makes a swap disk when partition is swap", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="swap" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemSwap)

			Expect(1).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
		})
	})

	Describe("when using ext4 fs", func() {
		It("allows lazy itable support", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeFs.WriteFile("/sys/fs/ext4/features/lazy_itable_init", []byte{})
			fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda2", FileSystemExt4)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", "/dev/xvda2"}))

		})

		It("allows without lazy itable support", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda2", FileSystemExt4)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "/dev/xvda2"}))
		})

		It("does not re-partition if fs is already ext4", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemExt4)

			Expect(1).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
		})
	})

	Describe("when using xfs fs", func() {
		It("formats with type xfs", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda2", FileSystemXFS)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mkfs.xfs", "/dev/xvda2"}))
		})

		It("does not re-partition if fs is already xfs", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="xfs" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemXFS)

			Expect(1).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
		})

		It("throws an error if formatting filesystem fails", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("mkfs.xfs /dev/xvda2", fakesys.FakeCmdResult{Error: errors.New("Sadness")})
			fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			err := formatter.Format("/dev/xvda2", FileSystemXFS)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Shelling out to mkfs.xfs: Sadness"))
		})
	})
})
