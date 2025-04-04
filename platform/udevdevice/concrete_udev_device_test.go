package udevdevice_test

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice"
)

var _ = Describe("ConcreteUdevDevice", func() {
	var (
		cmdRunner *fakes.FakeCmdRunner
		udev      ConcreteUdevDevice
		logger    boshlog.Logger
	)

	BeforeEach(func() {
		cmdRunner = fakes.NewFakeCmdRunner()
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	JustBeforeEach(func() {
		udev = NewConcreteUdevDevice(cmdRunner, logger)
	})

	Describe("#Settle", func() {
		Context("if `udevadm` is a runnable command", func() {
			BeforeEach(func() {
				cmdRunner.AvailableCommands["udevadm"] = true
			})

			It("runs `udevadm settle`", func() {
				err := udev.Settle()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"udevadm", "settle"}))
			})
		})

		Context("if `udevsettle` is a runnable command", func() {
			BeforeEach(func() {
				cmdRunner.AvailableCommands["udevsettle"] = true
			})

			It("runs `udevsettle`", func() {
				err := udev.Settle()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"udevsettle"}))
			})
		})

		Context("if neither `udevadm` nor `udevsettle` exist", func() {
			It("errors", func() {
				err := udev.Settle()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("can not find udevadm or udevsettle commands"))
			})
		})
	})

	Describe("#Trigger", func() {
		Context("if `udevadm` is a runnable command", func() {
			BeforeEach(func() {
				cmdRunner.AvailableCommands["udevadm"] = true
			})

			It("runs `udevadm trigger`", func() {
				err := udev.Trigger()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"udevadm", "trigger"}))
			})
		})

		Context("if `udevtrigger` is a runnable command", func() {
			BeforeEach(func() {
				cmdRunner.AvailableCommands["udevtrigger"] = true
			})

			It("runs `udevtrigger`", func() {
				err := udev.Trigger()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cmdRunner.RunCommands)).To(Equal(1))
				Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"udevtrigger"}))
			})
		})

		Context("if neither `udevadm` nor `udevtrigger` exist", func() {
			It("errors", func() {
				err := udev.Trigger()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("can not find udevadm or udevtrigger commands"))
			})
		})
	})
})
