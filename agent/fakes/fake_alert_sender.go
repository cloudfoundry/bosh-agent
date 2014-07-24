package fakes

import (
	boshalert "github.com/cloudfoundry/bosh-agent/agent/alert"
	boshsyslog "github.com/cloudfoundry/bosh-agent/syslog"
)

type FakeAlertSender struct {
	SendAlertMonitAlert boshalert.MonitAlert
	SendAlertErr        error

	SendSSHAlertMsg boshsyslog.Msg
	SendSSHAlertErr error
}

func (as *FakeAlertSender) SendAlert(monitAlert boshalert.MonitAlert) error {
	as.SendAlertMonitAlert = monitAlert
	return as.SendAlertErr
}

func (as *FakeAlertSender) SendSSHAlert(msg boshsyslog.Msg) error {
	as.SendSSHAlertMsg = msg
	return as.SendSSHAlertErr
}
