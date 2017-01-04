// +build windows

package syslog

type WindowsSyslogger struct{}

func NewSysLogger() Logger {
	return &WindowsSyslogger{}
}

func (w *WindowsSyslogger) Debug(msg string) error {
	return nil
}

func (w *WindowsSyslogger) Err(msg string) error {
	return nil
}
