package net_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/platform/net"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("KernelIPv6", func() {
	var (
		fs         *fakesys.FakeFileSystem
		cmdRunner  *fakesys.FakeCmdRunner
		kernelIPv6 KernelIPv6
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		kernelIPv6 = NewKernelIPv6Impl(fs, cmdRunner, logger)
	})

	Describe("Enable", func() {
		var (
			stopCh chan struct{}
		)

		BeforeEach(func() {
			stopCh = make(chan struct{}, 1)
		})

		act := func() error { return kernelIPv6.Enable(stopCh) }

		Context("when grub.conf disables IPv6", func() {
			BeforeEach(func() {
				err := fs.WriteFileString("/boot/grub/grub.conf", "before ipv6.disable=1 after")
				Expect(err).ToNot(HaveOccurred())
			})

			It("removes ipv6.disable=1 from grub.conf", func() {
				stopCh <- struct{}{}
				Expect(act()).ToNot(HaveOccurred())
				Expect(fs.ReadFileString("/boot/grub/grub.conf")).To(Equal("before  after"))
			})

			It("reboots after changing grub.conf and continue waiting until reboot event succeeds", func() {
				stopCh <- struct{}{}
				Expect(act()).ToNot(HaveOccurred())
				Expect(cmdRunner.RunCommands).To(Equal([][]string{{"shutdown", "-r", "now"}}))
			})

			It("returns an error if it fails to read grub.conf", func() {
				fs.ReadFileError = errors.New("fake-err")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err"))
			})

			It("returns an error if update to grub.conf fails", func() {
				fs.WriteFileError = errors.New("fake-err")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err"))
			})

			It("returns an error if shutdown fails", func() {
				cmdRunner.AddCmdResult("shutdown -r now", fakesys.FakeCmdResult{
					Error: errors.New("fake-err"),
				})

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err"))
			})
		})

		Context("when grub.conf allows IPv6", func() {
			BeforeEach(func() {
				err := fs.WriteFileString("/boot/grub/grub.conf", "before after")
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not change grub.conf", func() {
				Expect(act()).ToNot(HaveOccurred())
				Expect(fs.ReadFileString("/boot/grub/grub.conf")).To(Equal("before after"))
			})

			It("does not reboot but sets IPv6 sysctl", func() {
				Expect(act()).ToNot(HaveOccurred())
				Expect(cmdRunner.RunCommands).To(Equal([][]string{
					{"sysctl", "net.ipv6.conf.all.accept_ra=1"},
					{"sysctl", "net.ipv6.conf.default.accept_ra=1"},
					{"sysctl", "net.ipv6.conf.all.disable_ipv6=0"},
					{"sysctl", "net.ipv6.conf.default.disable_ipv6=0"},
				}))
			})

			It("writes sysctl commands to disk", func() {
				Expect(act()).ToNot(HaveOccurred())
				Expect(fs.ReadFileString("/run/sysctl.d/70-bosh-sysctl.conf")).To(Equal(`net.ipv6.conf.all.accept_ra=1
net.ipv6.conf.default.accept_ra=1
net.ipv6.conf.all.disable_ipv6=0
net.ipv6.conf.default.disable_ipv6=0`))
			})

			It("fails if the underlying sysctl fails", func() {
				cmdRunner.AddCmdResult("sysctl net.ipv6.conf.all.accept_ra=1", fakesys.FakeCmdResult{
					Error: errors.New("fake-err"),
				})

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-err"))
			})
		})
	})
})
