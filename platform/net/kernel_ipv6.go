package net

import (
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type KernelIPv6 interface {
	Enable(stopCh <-chan struct{}) error
}

type KernelIPv6Impl struct {
	fs        boshsys.FileSystem
	cmdRunner boshsys.CmdRunner
	logger    boshlog.Logger
}

func NewKernelIPv6Impl(fs boshsys.FileSystem, cmdRunner boshsys.CmdRunner, logger boshlog.Logger) KernelIPv6Impl {
	return KernelIPv6Impl{fs: fs, cmdRunner: cmdRunner, logger: logger}
}

func (net KernelIPv6Impl) Enable(stopCh <-chan struct{}) error {
	const (
		grubConfPath       = "/boot/grub/grub.conf"
		grubIPv6DisableOpt = "ipv6.disable=1"
	)

	grubConf, err := net.fs.ReadFileString(grubConfPath)
	if err != nil {
		return bosherr.WrapError(err, "Reading grub")
	}

	if strings.Contains(grubConf, grubIPv6DisableOpt) {
		grubConf = strings.Replace(grubConf, grubIPv6DisableOpt, "", -1)

		err = net.fs.WriteFileString(grubConfPath, grubConf)
		if err != nil {
			return bosherr.WrapError(err, "Writing grub.conf")
		}

		net.logger.Info("net.KernelIPv6", "Rebooting to enable IPv6 in kernel")

		_, _, _, err = net.cmdRunner.RunCommand("shutdown", "-r", "now")
		if err != nil {
			return bosherr.WrapError(err, "Rebooting for IPv6")
		}

		// Wait here for the OS to reboot the machine
		<-stopCh

		return nil
	}

	ipv6Sysctls := []string{
		"net.ipv6.conf.all.accept_ra=1",
		"net.ipv6.conf.default.accept_ra=1",
		"net.ipv6.conf.all.disable_ipv6=0",
		"net.ipv6.conf.default.disable_ipv6=0",
	}

	for _, sysctl := range ipv6Sysctls {
		_, _, _, err := net.cmdRunner.RunCommand("sysctl", sysctl)
		if err != nil {
			return bosherr.WrapError(err, "Running IPv6 sysctl")
		}
	}

	err = net.fs.MkdirAll("/run/sysctl.d", 0755)
	if err != nil {
		return bosherr.WrapError(err, "Creating /run/sysctl.d")
	}

	err = net.fs.WriteFileString("/run/sysctl.d/70-bosh-sysctl.conf", strings.Join(ipv6Sysctls, "\n"))
	if err != nil {
		return bosherr.WrapError(err, "Writing to /run/sysctl.d/70-bosh-sysctl.conf")
	}

	return nil
}
