// +build !windows

package platform

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"log"
	"log/syslog"
)

type DelayedAuditLogger struct {
	debugAuditLogger       *log.Logger
	errAuditLogger         *log.Logger
	debugLogCh, errorLogCh chan string
	logger                 boshlog.Logger
}

const delayedAuditLoggerTag = "DelayedAuditLogger"

func NewDelayedAuditLogger(logger boshlog.Logger) *DelayedAuditLogger {
	debugAuditLogger, err := syslog.NewLogger(syslog.LOG_DEBUG, log.LstdFlags)
	if err != nil {
		logger.Error(delayedAuditLoggerTag, err.Error())
	}

	errAuditLogger, err := syslog.NewLogger(syslog.LOG_ERR, log.LstdFlags)
	if err != nil {
		logger.Error(delayedAuditLoggerTag, err.Error())
	}

	return &DelayedAuditLogger{
		debugLogCh:       make(chan string, 1000),
		errorLogCh:       make(chan string, 1000),
		debugAuditLogger: debugAuditLogger,
		errAuditLogger:   errAuditLogger,
		logger:           logger,
	}
}

func (l *DelayedAuditLogger) StartLogging() {
	if l.debugAuditLogger == nil || l.errAuditLogger == nil {
		return
	}

	l.logger.Debug(delayedAuditLoggerTag, "Starting logging to syslog...")

	go func() {
		for debugLog := range l.debugLogCh {
			l.debugAuditLogger.Print(debugLog)
		}
	}()

	go func() {
		for errorLog := range l.errorLogCh {
			l.errAuditLogger.Print(errorLog)
		}
	}()
}

func (l *DelayedAuditLogger) Debug(msg string) {
	l.logger.Debug(delayedAuditLoggerTag, "Logging %s to syslog", msg)

	l.debugLogCh <- msg
}

func (l *DelayedAuditLogger) Err(msg string) {
	l.logger.Debug(delayedAuditLoggerTag, "Logging %s to syslog", msg)

	l.errorLogCh <- msg
}
