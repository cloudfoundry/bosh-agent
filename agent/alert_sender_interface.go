package agent

import (
	boshalert "github.com/cloudfoundry/bosh-agent/agent/alert"
	boshsyslog "github.com/cloudfoundry/bosh-agent/syslog"
)

type AlertSender interface {
	SendAlert(boshalert.MonitAlert) error
	SendSSHAlert(boshsyslog.Msg) error
}
