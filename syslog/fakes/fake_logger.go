package fakes

import "github.com/cloudfoundry/bosh-agent/syslog"

type FakeSyslogger struct {
	DebugMsg string
	ErrMsg   string
}

func NewFakeSyslogger() syslog.Logger {
	return &FakeSyslogger{}
}

func (f *FakeSyslogger) Debug(msg string) error {
	f.DebugMsg = msg
	return nil
}

func (f *FakeSyslogger) Err(msg string) error {
	f.ErrMsg = msg
	return nil
}
