package disk_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/disk"
	"github.com/cloudfoundry/bosh-agent/platform/disk/diskfakes"
)

var _ = Describe("linuxBindMounter", func() {
	var (
		delegateErr     error
		delegateMounter *diskfakes.FakeMounter
		mounter         Mounter
	)

	BeforeEach(func() {
		delegateErr = errors.New("fake-err")
		delegateMounter = &diskfakes.FakeMounter{}
		mounter = NewLinuxBindMounter(delegateMounter)
	})

	Describe("MountFilesystem", func() {
		Context("when mounting regular directory", func() {
			It("delegates to mounter and adds 'bind' option to mount as a bind-mount", func() {
				delegateMounter.MountFilesystemReturns(delegateErr)

				err := mounter.MountFilesystem("fake-partition-path", "fake-mount-path", "awesomefs", "fake-opt1")

				// Outputs
				Expect(err).To(Equal(delegateErr))

				// Inputs
				Expect(delegateMounter.MountFilesystemCallCount()).To(Equal(1))
				partition, mntPt, fstype, options := delegateMounter.MountFilesystemArgsForCall(0)
				Expect(partition).To(Equal("fake-partition-path"))
				Expect(mntPt).To(Equal("fake-mount-path"))
				Expect(fstype).To(Equal("awesomefs"))
				Expect(options).To(Equal([]string{"fake-opt1", "bind"}))
			})
		})

		Context("when mounting tmpfs", func() {
			It("delegates to mounter and does not add 'bind' option to mount as a bind-mount", func() {
				delegateMounter.MountFilesystemReturns(delegateErr)

				err := mounter.MountFilesystem("somesrc", "fake-mount-path", "tmpfs", "fake-opt1")

				// Outputs
				Expect(err).To(Equal(delegateErr))

				// Inputs
				Expect(delegateMounter.MountFilesystemCallCount()).To(Equal(1))
				partition, mntPt, fstype, options := delegateMounter.MountFilesystemArgsForCall(0)
				Expect(partition).To(Equal("somesrc"))
				Expect(mntPt).To(Equal("fake-mount-path"))
				Expect(fstype).To(Equal("tmpfs"))
				Expect(options).To(Equal([]string{"fake-opt1"}))
			})
		})
	})

	Describe("Mount", func() {
		It("delegates to mounter with empty string filesystem type (so that it can be inferred)", func() {
			delegateMounter.MountFilesystemReturns(delegateErr)

			err := mounter.Mount("fake-partition-path", "fake-mount-path", "fake-opt1")

			// Outputs
			Expect(err).To(Equal(delegateErr))

			// Inputs
			Expect(delegateMounter.MountFilesystemCallCount()).To(Equal(1))
			partition, mntPt, fstype, options := delegateMounter.MountFilesystemArgsForCall(0)
			Expect(partition).To(Equal("fake-partition-path"))
			Expect(mntPt).To(Equal("fake-mount-path"))
			Expect(fstype).To(Equal(""))
			Expect(options).To(Equal([]string{"fake-opt1", "bind"}))
		})
	})

	Describe("RemountAsReadonly", func() {
		It("does not delegate to mounter because remount with 'bind' does not work", func() {
			err := mounter.RemountAsReadonly("fake-path")
			Expect(err).To(BeNil())
			Expect(delegateMounter.RemountAsReadonlyCallCount()).To(Equal(0))
		})
	})

	Describe("Remount", func() {
		It("delegates to mounter and adds 'bind' option to mount as a bind-mount", func() {
			delegateMounter.RemountReturns(delegateErr)

			err := mounter.Remount("fake-from-path", "fake-to-path", "fake-opt1")

			// Outputs
			Expect(err).To(Equal(delegateErr))

			// Inputs
			Expect(delegateMounter.RemountCallCount()).To(Equal(1))
			fromPath, toPath, options := delegateMounter.RemountArgsForCall(0)
			Expect(fromPath).To(Equal("fake-from-path"))
			Expect(toPath).To(Equal("fake-to-path"))
			Expect(options).To(Equal([]string{"fake-opt1", "bind"}))
		})
	})

	Describe("SwapOn", func() {
		It("delegates to mounter", func() {
			delegateMounter.SwapOnReturns(delegateErr)

			err := mounter.SwapOn("fake-path")

			// Outputs
			Expect(err).To(Equal(delegateErr))

			// Inputs
			Expect(delegateMounter.SwapOnCallCount()).To(Equal(1))
			Expect(delegateMounter.SwapOnArgsForCall(0)).To(Equal("fake-path"))
		})
	})

	Describe("Unmount", func() {
		It("delegates to mounter", func() {
			delegateMounter.UnmountReturns(true, delegateErr)

			didUnmount, err := mounter.Unmount("fake-device-path")

			// Outputs
			Expect(didUnmount).To(BeTrue())
			Expect(err).To(Equal(delegateErr))

			// Inputs
			Expect(delegateMounter.UnmountCallCount()).To(Equal(1))
			Expect(delegateMounter.UnmountArgsForCall(0)).To(Equal("fake-device-path"))
		})
	})

	Describe("IsMountPoint", func() {
		It("delegates to mounter", func() {
			delegateMounter.IsMountPointReturns("fake-partition-path", true, delegateErr)

			partitionPath, isMountPoint, err := mounter.IsMountPoint("fake-device-path")

			// Outputs
			Expect(partitionPath).To(Equal("fake-partition-path"))
			Expect(isMountPoint).To(BeTrue())
			Expect(err).To(Equal(delegateErr))

			// Inputs
			Expect(delegateMounter.IsMountPointCallCount()).To(Equal(1))
			Expect(delegateMounter.IsMountPointArgsForCall(0)).To(Equal("fake-device-path"))
		})
	})

	Describe("IsMounted", func() {
		It("delegates to mounter", func() {
			delegateMounter.IsMountedReturns(true, delegateErr)

			isMounted, err := mounter.IsMounted("fake-device-path")

			// Outputs
			Expect(isMounted).To(BeTrue())
			Expect(err).To(Equal(delegateErr))

			// Inputs
			Expect(delegateMounter.IsMountedArgsForCall(0)).To(Equal("fake-device-path"))
		})
	})
})
