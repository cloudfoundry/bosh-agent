package disk_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"errors"
	"fmt"

	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("Linux Formatter", func() {
	Describe("Format", func() {
		Context("when using swap", func() {
			It("format as swap disk if partition has not been formatted", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{ExitStatus: 2, Error: errors.New("Exit code 2")})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda2", FileSystemSwap)
				Expect(err).NotTo(HaveOccurred())

				Expect(2).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mkswap", "/dev/xvda2"}))
			})

			It("reformats the partition if is not formatted as swap", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda1", FileSystemSwap)
				Expect(err).NotTo(HaveOccurred())

				Expect(2).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mkswap", "/dev/xvda1"}))
			})

			It("does not reformat if it already formatted as swap", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="swap" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda1", FileSystemSwap)
				Expect(err).NotTo(HaveOccurred())

				Expect(1).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
			})
		})

		Context("when using ext4", func() {
			It("allows lazy itable support", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				err := fakeFs.WriteFile("/sys/fs/ext4/features/lazy_itable_init", []byte{})
				Expect(err).NotTo(HaveOccurred())
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err = formatter.Format("/dev/xvda2", FileSystemExt4)
				Expect(err).NotTo(HaveOccurred())

				Expect(2).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", "/dev/xvda2"}))
			})

			Context("when mke2fs errors", func() {
				var fakeRunner *fakesys.FakeCmdRunner
				var fakeFs *fakesys.FakeFileSystem
				var mkeCmd string

				BeforeEach(func() {
					fakeRunner = fakesys.NewFakeCmdRunner()
					fakeFs = fakesys.NewFakeFileSystem()
					err := fakeFs.WriteFile("/sys/fs/ext4/features/lazy_itable_init", []byte{})
					Expect(err).NotTo(HaveOccurred())
					fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

					mkeCmd = fmt.Sprintf("mke2fs -t %s -j -E lazy_itable_init=1 %s", FileSystemExt4, "/dev/xvda2")
				})

				It("retries mke2fs if the erros is 'device is already in use'", func() {
					fakeRunner.AddCmdResult(mkeCmd, fakesys.FakeCmdResult{
						Error: errors.New(`mke2fs 1.42.9 (4-Feb-2014)
/dev/xvdf1 is apparently in use by the system; will not make a filesystem here`),
					})
					fakeRunner.AddCmdResult(mkeCmd, fakesys.FakeCmdResult{
						ExitStatus: 0,
					})
					formatter := NewLinuxFormatter(fakeRunner, fakeFs)
					err := formatter.Format("/dev/xvda2", FileSystemExt4)
					Expect(err).NotTo(HaveOccurred())

					Expect(3).To(Equal(len(fakeRunner.RunCommands)))
					Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", "/dev/xvda2"}))
					Expect(fakeRunner.RunCommands[2]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", "/dev/xvda2"}))
				})

				It("does not retry and returns the error otherwise", func() {
					fakeRunner.AddCmdResult(mkeCmd, fakesys.FakeCmdResult{
						Error: errors.New(`some other error`),
					})
					formatter := NewLinuxFormatter(fakeRunner, fakeFs)
					err := formatter.Format("/dev/xvda2", FileSystemExt4)
					Expect(err).To(HaveOccurred())

					Expect(2).To(Equal(len(fakeRunner.RunCommands)))
					Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "-E", "lazy_itable_init=1", "/dev/xvda2"}))
				})
			})

			It("allows without lazy itable support", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext2" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda2", FileSystemExt4)
				Expect(err).NotTo(HaveOccurred())

				Expect(2).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mke2fs", "-t", "ext4", "-j", "/dev/xvda2"}))
			})

			It("does not re-partition if fs is already ext4", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda1", FileSystemExt4)
				Expect(err).NotTo(HaveOccurred())

				Expect(1).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
			})

			It("does not re-partition if fs is already xfs", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="xfs" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda2", FileSystemExt4)
				Expect(err).NotTo(HaveOccurred())

				Expect(1).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda2"}))
			})

			It("reformats if fs is not a supported fs type", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="somethingelse" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda2", FileSystemExt4)
				Expect(err).NotTo(HaveOccurred())

				Expect(2).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda2"}))
			})
		})

		Context("when using xfs", func() {
			It("formats a blank disk with type xfs", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{ExitStatus: 2, Error: errors.New("Exit code 2")})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda2", FileSystemXFS)
				Expect(err).NotTo(HaveOccurred())

				Expect(2).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"mkfs.xfs", "/dev/xvda2"}))
			})

			It("does not re-format if fs is already ext4", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda1", FileSystemXFS)
				Expect(err).NotTo(HaveOccurred())

				Expect(1).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
			})

			It("does not re-partition if fs is already xfs", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("blkid -p /dev/xvda1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="xfs" yyyy zzzz`})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda1", FileSystemXFS)
				Expect(err).NotTo(HaveOccurred())

				Expect(1).To(Equal(len(fakeRunner.RunCommands)))
				Expect(fakeRunner.RunCommands[0]).To(Equal([]string{"blkid", "-p", "/dev/xvda1"}))
			})

			It("throws an error if formatting filesystem fails", func() {
				fakeRunner := fakesys.NewFakeCmdRunner()
				fakeFs := fakesys.NewFakeFileSystem()
				fakeRunner.AddCmdResult("mkfs.xfs /dev/xvda2", fakesys.FakeCmdResult{Error: errors.New("Sadness")})
				fakeRunner.AddCmdResult("blkid -p /dev/xvda2", fakesys.FakeCmdResult{Stderr: "", ExitStatus: 2})

				formatter := NewLinuxFormatter(fakeRunner, fakeFs)
				err := formatter.Format("/dev/xvda2", FileSystemXFS)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Shelling out to mkfs.xfs: Sadness"))
			})
		})
	})

	Describe("GrowFilesystem", func() {
		var (
			fakeRunner *fakesys.FakeCmdRunner
			fakeFs     *fakesys.FakeFileSystem
			formatter  Formatter
		)

		BeforeEach(func() {
			fakeRunner = fakesys.NewFakeCmdRunner()
			fakeFs = fakesys.NewFakeFileSystem()
		})

		Context("when determining partition filesystem fails", func() {
			BeforeEach(func() {
				fakeRunner.AddCmdResult("blkid -p /dev/nvme2n1p1", fakesys.FakeCmdResult{ExitStatus: 1, Error: errors.New("No GPT found")})
				formatter = NewLinuxFormatter(fakeRunner, fakeFs)
			})

			It("returns an error", func() {
				err := formatter.GrowFilesystem("/dev/nvme2n1p1")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("No GPT found"))
			})
		})

		Context("when using Ext4", func() {
			BeforeEach(func() {
				fakeRunner.AddCmdResult("blkid -p /dev/nvme2n1p1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="ext4" yyyy zzzz`})
				formatter = NewLinuxFormatter(fakeRunner, fakeFs)
			})

			It("grows the Ext4 filesystem", func() {
				err := formatter.GrowFilesystem("/dev/nvme2n1p1")

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"resize2fs", "-f", "/dev/nvme2n1p1"}))
			})

			Context("when resize2fs fails", func() {
				BeforeEach(func() {
					fakeRunner.AddCmdResult("resize2fs -f /dev/nvme2n1p1", fakesys.FakeCmdResult{ExitStatus: 1, Error: errors.New("resize2fs failure")})
				})

				It("returns an error", func() {
					err := formatter.GrowFilesystem("/dev/nvme2n1p1")

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Failed to grow Ext4 filesystem"))
					Expect(err.Error()).To(ContainSubstring("resize2fs failure"))
				})
			})
		})

		Context("when using XFS", func() {
			BeforeEach(func() {
				fakeRunner.AddCmdResult("blkid -p /dev/nvme2n1p1", fakesys.FakeCmdResult{Stdout: `xxxxx TYPE="xfs" yyyy zzzz`})
				formatter = NewLinuxFormatter(fakeRunner, fakeFs)
			})

			It("grows the XFS filesystem", func() {
				err := formatter.GrowFilesystem("/dev/nvme2n1p1")

				Expect(err).NotTo(HaveOccurred())
				Expect(fakeRunner.RunCommands[1]).To(Equal([]string{"xfs_growfs", "/dev/nvme2n1p1"}))
			})

			Context("when xfs_growfs fails", func() {
				BeforeEach(func() {
					fakeRunner.AddCmdResult("xfs_growfs /dev/nvme2n1p1", fakesys.FakeCmdResult{ExitStatus: 1, Error: errors.New("xfs_growfs failure")})
				})

				It("returns an error", func() {
					err := formatter.GrowFilesystem("/dev/nvme2n1p1")

					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Failed to grow XFS filesystem"))
					Expect(err.Error()).To(ContainSubstring("xfs_growfs failure"))
				})
			})
		})
	})
})
