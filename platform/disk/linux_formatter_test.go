package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() {
	Describe("Testing with Ginkgo", func() {
		It("linux format when using swap fs", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemSwap)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mkswap", "/dev/xvda1"}))
		})
		It("linux format when using swap fs and partition is swap", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="swap" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemSwap)

			Expect(1).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
		})
		It("linux format when using ext4 fs with lazy itable support", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeFs.WriteFile("/sys/fs/ext4/features/lazy_itable_init", []byte{})
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda2", FileSystemExt4)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", "/dev/xvda2"}))
		})
		It("linux format when using ext4 fs without lazy itable support", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda2", FileSystemExt4)

			Expect(2).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "/dev/xvda2"}))
		})
		It("linux format when using ext4 fs and partition is ext4", func() {

			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			formatter.Format("/dev/xvda1", FileSystemExt4)

			Expect(1).To(Equal(len(fakeRunner.RunCommands)))
			Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
		})
		It("linux format when unable to detect partition type", func() {
			fakeRunner := fakesys.NewFakeCmdRunner()
			fakeFs := fakesys.NewFakeFileSystem()
			fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Error: errors.New("command not found")})

			formatter := NewLinuxFormatter(fakeRunner, fakeFs)
			err := formatter.Format("/dev/xvda1", FileSystemExt4)

			Expect(err).To(HaveOccurred())
			Expect(1).To(Equal(len(fakeRunner.RunCommands)))
		})
	})
}
