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
		grubConfPathBIOS            = "/boot/grub/grub.cfg"
		grubConfPathEFI             = "/boot/efi/EFI/grub/grub.cfg"
		grubIPv6DisableOpt          = "ipv6.disable=1"
		boshSysctlPath              = "/etc/sysctl.d/60-bosh-sysctl.conf"
		sysctlIpv6AllDisableOpt     = "net.ipv6.conf.all.disable_ipv6=1"
		sysctlIpv6DefaultDisableOpt = "net.ipv6.conf.default.disable_ipv6=1"
		sysctlIpv6AllEnableOpt      = "net.ipv6.conf.all.disable_ipv6=0"
		sysctlIpv6DefaultEnableOpt  = "net.ipv6.conf.default.disable_ipv6=0"
	)

	boshSysctl, err := net.fs.ReadFileString(boshSysctlPath)
	if err != nil {
		return bosherr.WrapError(err, "Reading boshSysctl")
	}

	if strings.Contains(boshSysctl, sysctlIpv6AllDisableOpt) {
		boshSysctl = strings.ReplaceAll(boshSysctl, sysctlIpv6AllDisableOpt, sysctlIpv6AllEnableOpt)
		boshSysctl = strings.ReplaceAll(boshSysctl, sysctlIpv6DefaultDisableOpt, sysctlIpv6DefaultEnableOpt)
		err = net.fs.WriteFileString(boshSysctlPath, boshSysctl)
		if err != nil {
			return bosherr.WrapError(err, "Writing boshSysctl")
		}
	}

	grubConfPath := grubConfPathBIOS

	grubConf, err := net.fs.ReadFileString(grubConfPath)
	if err != nil {
		grubConfPath = grubConfPathEFI
		grubConf, err = net.fs.ReadFileString(grubConfPath)
		if err != nil {
			return bosherr.WrapError(err, "Reading grub")
		}
	}

	if strings.Contains(grubConf, grubIPv6DisableOpt) {
		grubConf = strings.ReplaceAll(grubConf, grubIPv6DisableOpt, "")

		err = net.fs.WriteFileString(grubConfPath, grubConf)
		if err != nil {
			return bosherr.WrapError(err, "Writing grub.cnf")
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
		sysctlIpv6AllEnableOpt,
		sysctlIpv6DefaultEnableOpt,
	}

	for _, sysctl := range ipv6Sysctls {
		_, _, _, err := net.cmdRunner.RunCommand("sysctl", sysctl)
		if err != nil {
			return bosherr.WrapError(err, "Running IPv6 sysctl")
		}
	}

	return nil
}
