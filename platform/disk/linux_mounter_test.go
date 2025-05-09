package disk_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	fakedisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk/fakes"
)

type changingMountsSearcher struct {
	mounts [][]Mount
}

func (s *changingMountsSearcher) SearchMounts() ([]Mount, error) {
	result := s.mounts[0]
	s.mounts = s.mounts[1:]
	return result, nil
}

const swaponUsageOutput = `Filename				Type		Size	Used	Priority
/dev/swap                              partition	78180316	0	-1
`

const swaponUsageOutputWithOtherDevice = `Filename				Type		Size	Used	Priority
/dev/swap2                              partition	78180316	0	-1
`

var _ = Describe("linuxMounter", func() {
	var (
		runner         *fakesys.FakeCmdRunner
		mountsSearcher *fakedisk.FakeMountsSearcher
		mounter        Mounter
	)

	BeforeEach(func() {
		runner = fakesys.NewFakeCmdRunner()
		mountsSearcher = &fakedisk.FakeMountsSearcher{}
		mounter = NewLinuxMounter(runner, mountsSearcher, 1*time.Millisecond)
	})

	Describe("MountTmpfs", func() {
		Context("when the tmpfs has not been mounted yet", func() {
			BeforeEach(func() {
				mountsSearcher.SearchMountsMounts = []Mount{
					Mount{PartitionPath: "/dev/bob1", MountPoint: "/mnt/bob"},
				}
			})

			It("mounts it", func() {
				err := mounter.MountTmpfs("/mnt/joe", "16mb")
				Expect(err).ToNot(HaveOccurred())

				Expect(1).To(Equal(len(runner.RunCommands)))
				Expect(runner.RunCommands[0]).To(Equal([]string{"mount", "tmpfs", "/mnt/joe", "-t", "tmpfs", "-o", "size=16mb"}))
			})
		})

		Context("when the tmpfs is already mounted", func() {
			BeforeEach(func() {
				mountsSearcher.SearchMountsMounts = []Mount{
					Mount{PartitionPath: "/dev/bob1", MountPoint: "/mnt/bob"},
					Mount{PartitionPath: "/dev/joe1", MountPoint: "/mnt/joe"},
				}
			})

			It("does not mount again", func() {
				err := mounter.MountTmpfs("/mnt/joe", "16mb")
				Expect(err).ToNot(HaveOccurred())

				Expect(0).To(Equal(len(runner.RunCommands)))
			})
		})

		Context("When searching for mounts returns an error", func() {
			It("wraps the error", func() {
				mountsSearcher.SearchMountsErr = errors.New("u crazy fam")

				err := mounter.MountTmpfs("/mnt/joe", "16mb")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Searching mounts: u crazy fam"))
			})
		})

		Context("When mounting tmpfs returns an error", func() {
			BeforeEach(func() {
				runner.AddCmdResult("mount tmpfs /mnt/joe -t tmpfs -o size=16mb", fakesys.FakeCmdResult{Error: errors.New("KAZAM")})
			})

			It("wraps the error", func() {
				err := mounter.MountTmpfs("/mnt/joe", "16mb")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Mounting tmpfs to /mnt/joe: Shelling out to mount: KAZAM"))
			})
		})
	})

	Describe("MountFilesystem", func() {
		It("allows to mount disk at given mount point", func() {
			err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "goodfs")
			Expect(err).ToNot(HaveOccurred())
			Expect(1).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"mount", "/dev/foo", "/mnt/foo", "-t", "goodfs"}))
		})

		Context("when there are mount options", func() {
			It("mounts the disk with specified options", func() {
				err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "goodfs", "opt1", "opt2")
				Expect(err).ToNot(HaveOccurred())
				Expect(1).To(Equal(len(runner.RunCommands)))
				Expect(runner.RunCommands[0]).To(Equal([]string{"mount", "/dev/foo", "/mnt/foo", "-t", "goodfs", "-o", "opt1", "-o", "opt2"}))
			})
		})

		It("does not try to mount disk again when disk is already mounted to the expected mount point", func() {
			mountsSearcher.SearchMountsMounts = []Mount{
				Mount{PartitionPath: "/dev/foo", MountPoint: "/mnt/foo"},
				Mount{PartitionPath: "/dev/bar", MountPoint: "/mnt/bar"},
			}

			err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "goodfs")
			Expect(err).ToNot(HaveOccurred())
			Expect(0).To(Equal(len(runner.RunCommands)))
		})

		It("returns error when disk is already mounted to the wrong mount point", func() {
			mountsSearcher.SearchMountsMounts = []Mount{
				Mount{PartitionPath: "/dev/foo", MountPoint: "/mnt/foobarbaz"},
				Mount{PartitionPath: "/dev/bar", MountPoint: "/mnt/bar"},
			}

			err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "goodfs")
			Expect(err).To(HaveOccurred())
			Expect(0).To(Equal(len(runner.RunCommands)))
		})

		It("allows to mount tmpfs to multiple mount points", func() {
			mountsSearcher.SearchMountsMounts = []Mount{
				Mount{PartitionPath: "tmpfs", MountPoint: "/mnt/foo1"},
			}

			err := mounter.MountFilesystem("tmpfs", "/mnt/foo2", "tmpfs")
			Expect(err).ToNot(HaveOccurred())
			Expect(1).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"mount", "tmpfs", "/mnt/foo2", "-t", "tmpfs"}))
		})

		It("returns error when another disk is already mounted to mount point", func() {
			mountsSearcher.SearchMountsMounts = []Mount{
				Mount{PartitionPath: "/dev/baz", MountPoint: "/mnt/foo"},
				Mount{PartitionPath: "/dev/bar", MountPoint: "/mnt/bar"},
			}

			err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "goodfs")
			Expect(err).To(HaveOccurred())
			Expect(0).To(Equal(len(runner.RunCommands)))
		})

		It("returns error and does not try to mount anything when searching mounts fails", func() {
			mountsSearcher.SearchMountsErr = errors.New("fake-search-mounts-err")

			err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "goodfs")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-search-mounts-err"))
			Expect(0).To(Equal(len(runner.RunCommands)))
		})

		Context("when the filesystem type is empty", func() {
			It("mounts the disk without filesystem type (so that it can be inferred)", func() {
				err := mounter.MountFilesystem("/dev/foo", "/mnt/foo", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(1).To(Equal(len(runner.RunCommands)))
				Expect(runner.RunCommands[0]).To(Equal([]string{"mount", "/dev/foo", "/mnt/foo"}))
			})
		})
	})

	Describe("Mount", func() {
		It("mounts the disk without filesystem type (so that it can be inferred)", func() {
			err := mounter.Mount("/dev/foo", "/mnt/foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(1).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"mount", "/dev/foo", "/mnt/foo"}))
		})
	})

	Describe("RemountInPlace", func() {
		Context("when the mount exists", func() {
			BeforeEach(func() {
				err := mounter.Mount("/mnt/foo", "/mnt/foo", "bind")
				Expect(err).ToNot(HaveOccurred())

				mountsSearcher.SearchMountsMounts = []Mount{
					Mount{PartitionPath: "/mnt/foo", MountPoint: "/mnt/foo"},
				}
			})

			It("remounts in place", func() {
				err := mounter.RemountInPlace("/mnt/foo", "nodev")
				Expect(err).ToNot(HaveOccurred())
				Expect(runner.RunCommands).To(HaveLen(2))
				Expect(runner.RunCommands[1]).To(Equal([]string{"mount", "", "/mnt/foo", "-o", "remount", "-o", "nodev"}))
			})
		})

		Context("when the mount does not exist", func() {
			It("raises error", func() {
				err := mounter.RemountInPlace("/mnt/foo", "remount,nodev")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Error finding existing mount point /mnt/foo"))
			})
		})

	})

	Describe("RemountAsReadonly", func() {
		It("remount as readonly", func() {
			changingMountsSearcher := &changingMountsSearcher{
				[][]Mount{
					[]Mount{Mount{PartitionPath: "/dev/baz", MountPoint: "/mnt/bar"}},
					[]Mount{Mount{PartitionPath: "/dev/baz", MountPoint: "/mnt/bar"}},
					[]Mount{},
				},
			}

			mounter := NewLinuxMounter(runner, changingMountsSearcher, 1*time.Millisecond)

			err := mounter.RemountAsReadonly("/mnt/bar")
			Expect(err).ToNot(HaveOccurred())
			Expect(2).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"umount", "/mnt/bar"}))
			Expect(runner.RunCommands[1]).To(Equal([]string{"mount", "/dev/baz", "/mnt/bar", "-o", "ro"}))
		})

		It("returns error and does not try to unmount/mount anything when searching mounts fails", func() {
			mountsSearcher.SearchMountsErr = errors.New("fake-search-mounts-err")

			err := mounter.RemountAsReadonly("/mnt/bar")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-search-mounts-err"))
			Expect(0).To(Equal(len(runner.RunCommands)))
		})
	})

	Describe("Remount", func() {
		It("remount", func() {
			changingMountsSearcher := &changingMountsSearcher{
				[][]Mount{
					[]Mount{Mount{PartitionPath: "/dev/baz", MountPoint: "/mnt/foo"}},
					[]Mount{Mount{PartitionPath: "/dev/baz", MountPoint: "/mnt/foo"}},
					[]Mount{},
				},
			}

			mounter := NewLinuxMounter(runner, changingMountsSearcher, 1*time.Millisecond)

			err := mounter.Remount("/mnt/foo", "/mnt/bar")
			Expect(err).ToNot(HaveOccurred())
			Expect(2).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"umount", "/mnt/foo"}))
			Expect(runner.RunCommands[1]).To(Equal([]string{"mount", "/dev/baz", "/mnt/bar"}))
		})

		It("returns error and does not try to unmount/mount anything when searching mounts fails", func() {
			mountsSearcher.SearchMountsErr = errors.New("fake-search-mounts-err")

			err := mounter.Remount("/mnt/foo", "/mnt/bar")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-search-mounts-err"))
			Expect(0).To(Equal(len(runner.RunCommands)))
		})
	})

	Describe("SwapOn", func() {
		It("linux swap on", func() {
			runner.AddCmdResult("swapon -s", fakesys.FakeCmdResult{Stdout: "Filename				Type		Size	Used	Priority\n"})

			err := mounter.SwapOn("/dev/swap")
			Expect(err).NotTo(HaveOccurred())
			Expect(2).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[1]).To(Equal([]string{"swapon", "/dev/swap"}))
		})

		It("linux swap on when already on", func() {
			runner.AddCmdResult("swapon -s", fakesys.FakeCmdResult{Stdout: swaponUsageOutput})

			err := mounter.SwapOn("/dev/swap")
			Expect(err).NotTo(HaveOccurred())
			Expect(1).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"swapon", "-s"}))
		})

		It("linux swap on when already on other device", func() {
			runner.AddCmdResult("swapon -s", fakesys.FakeCmdResult{Stdout: swaponUsageOutputWithOtherDevice})

			err := mounter.SwapOn("/dev/swap")
			Expect(err).NotTo(HaveOccurred())
			Expect(2).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"swapon", "-s"}))
			Expect(runner.RunCommands[1]).To(Equal([]string{"swapon", "/dev/swap"}))
		})
	})

	Describe("Unmount", func() {
		BeforeEach(func() {
			mountsSearcher.SearchMountsMounts = []Mount{
				Mount{PartitionPath: "/dev/xvdb2", MountPoint: "/var/vcap/data"},
			}
		})

		It("unmounts based on partition when partition is mounted", func() {
			didUnmount, err := mounter.Unmount("/dev/xvdb2")
			Expect(err).ToNot(HaveOccurred())
			Expect(didUnmount).To(BeTrue())

			Expect(1).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"umount", "/dev/xvdb2"}))
		})

		It("unmount based on mount point when mount point is mounted", func() {
			didUnmount, err := mounter.Unmount("/var/vcap/data")
			Expect(err).ToNot(HaveOccurred())
			Expect(didUnmount).To(BeTrue())

			Expect(1).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"umount", "/var/vcap/data"}))
		})

		It("returns without an error indicating that nothing was unmounted when partition or mount point is not mounted", func() {
			didUnmount, err := mounter.Unmount("/dev/xvdb3")
			Expect(err).ToNot(HaveOccurred())
			Expect(didUnmount).To(BeFalse())

			Expect(0).To(Equal(len(runner.RunCommands)))
		})

		It("returns without an error after failing several times and then succeeding to unmount", func() {
			runner.AddCmdResult("umount /dev/xvdb2", fakesys.FakeCmdResult{Error: errors.New("fake-error")})
			runner.AddCmdResult("umount /dev/xvdb2", fakesys.FakeCmdResult{Error: errors.New("fake-error")})
			runner.AddCmdResult("umount /dev/xvdb2", fakesys.FakeCmdResult{})

			didUnmount, err := mounter.Unmount("/dev/xvdb2")
			Expect(err).ToNot(HaveOccurred())
			Expect(didUnmount).To(BeTrue())

			Expect(3).To(Equal(len(runner.RunCommands)))
			Expect(runner.RunCommands[0]).To(Equal([]string{"umount", "/dev/xvdb2"}))
			Expect(runner.RunCommands[1]).To(Equal([]string{"umount", "/dev/xvdb2"}))
			Expect(runner.RunCommands[2]).To(Equal([]string{"umount", "/dev/xvdb2"}))
		})

		It("returns error when it fails to unmount too many times", func() {
			runner.AddCmdResult("umount /dev/xvdb2", fakesys.FakeCmdResult{Error: errors.New("fake-error"), Sticky: true})

			_, err := mounter.Unmount("/dev/xvdb2")
			Expect(err).To(HaveOccurred())
		})

		It("returns error and does not try to unmount anything when searching mounts fails", func() {
			mountsSearcher.SearchMountsErr = errors.New("fake-search-mounts-err")

			_, err := mounter.Unmount("/dev/xvdb2")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-search-mounts-err"))
			Expect(0).To(Equal(len(runner.RunCommands)))
		})
	})

	Describe("IsMountPoint", func() {
		Context("when it is a mount point", func() {
			It("is mount point", func() {
				mountsSearcher.SearchMountsMounts = []Mount{
					Mount{PartitionPath: "/dev/xvdb2", MountPoint: "/var/vcap/data"},
				}

				partitionPath, isMountPoint, err := mounter.IsMountPoint("/var/vcap/data")
				Expect(err).ToNot(HaveOccurred())
				Expect(partitionPath).To(Equal("/dev/xvdb2"))
				Expect(isMountPoint).To(BeTrue())
			})
		})

		Context("when it is not a mount point", func() {
			It("return empty partition path", func() {
				mountsSearcher.SearchMountsMounts = []Mount{
					Mount{PartitionPath: "/dev/xvdb2", MountPoint: "/var/vcap/data"},
				}

				partitionPath, isMountPoint, err := mounter.IsMountPoint("/var/vcap/store")
				Expect(err).ToNot(HaveOccurred())
				Expect(partitionPath).To(Equal(""))
				Expect(isMountPoint).To(BeFalse())
			})
		})

		Context("when searching mounts fails", func() {
			It("returns error", func() {
				mountsSearcher.SearchMountsErr = errors.New("fake-search-mounts-err")

				_, _, err := mounter.IsMountPoint("/var/vcap/store")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-search-mounts-err"))
			})
		})
	})

	Describe("IsMounted", func() {
		It("is mounted", func() {
			mountsSearcher.SearchMountsMounts = []Mount{
				Mount{PartitionPath: "/dev/xvdb2", MountPoint: "/var/vcap/data"},
			}

			isMounted, err := mounter.IsMounted("/dev/xvdb2")
			Expect(err).ToNot(HaveOccurred())
			Expect(isMounted).To(BeTrue())

			isMounted, err = mounter.IsMounted("/var/vcap/data")
			Expect(err).ToNot(HaveOccurred())
			Expect(isMounted).To(BeTrue())

			isMounted, err = mounter.IsMounted("/var/foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(isMounted).To(BeFalse())
		})

		It("returns error when searching mounts fails", func() {
			mountsSearcher.SearchMountsErr = errors.New("fake-search-mounts-err")

			_, err := mounter.IsMounted("/var/foo")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-search-mounts-err"))
		})
	})
})
