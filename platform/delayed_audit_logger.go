// +build !windows

package platform

import (
	"log"
	"log/syslog"
)

type DelayedAuditLogger struct {
	debugSyslogger *log.Logger
	errSyslogger   *log.Logger
}

func NewDelayedAuditLogger() *DelayedAuditLogger {
	return &DelayedAuditLogger{debugSyslogger: nil, errSyslogger: nil}
}

func (l *DelayedAuditLogger) Debug(msg string) error {
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

func (l *DelayedAuditLogger) Err(msg string) error {
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
