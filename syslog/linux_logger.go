// +build !windows

package syslog

import (
	"log"
	"log/syslog"
)

type LinuxSyslogger struct {
	debugSyslogger *log.Logger
	errSyslogger   *log.Logger
}

func NewSysLogger() Logger {
	return &LinuxSyslogger{debugSyslogger: nil, errSyslogger: nil}
}

func (l *LinuxSyslogger) Debug(msg string) error {
	if l.debugSyslogger == nil {
		debugSyslogger, err := syslog.NewLogger(syslog.LOG_DEBUG, log.LstdFlags)
		if err != nil {
			return err
		}
		l.debugSyslogger = debugSyslogger
	}

	l.debugSyslogger.Println(msg)
	return nil
}

func (l *LinuxSyslogger) Err(msg string) error {
	if l.errSyslogger == nil {
		errSyslogger, err := syslog.NewLogger(syslog.LOG_ERR, log.LstdFlags)
		if err != nil {
			return err
		}
		l.errSyslogger = errSyslogger
	}

	l.errSyslogger.Println(msg)
	return nil
}
