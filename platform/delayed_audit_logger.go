// +build !windows

package platform

import (
	"fmt"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	"log"
	"time"
)

type DelayedAuditLogger struct {
	debugAuditLogger       *log.Logger
	errAuditLogger         *log.Logger
	debugLogCh, errorLogCh chan string
	logger                 boshlog.Logger
}

const delayedAuditLoggerTag = "DelayedAuditLogger"

func NewDelayedAuditLogger(logger boshlog.Logger) *DelayedAuditLogger {
	return &DelayedAuditLogger{
		debugLogCh: make(chan string, 1000),
		errorLogCh: make(chan string, 1000),
		logger:     logger,
	}
}

func (l *DelayedAuditLogger) StartLogging(auditLoggerProvider AuditLoggerProvider) {
	go func() {
		retryable := boshretry.NewRetryable(func() (bool, error) {
			debugAuditLogger, err := auditLoggerProvider.ProvideDebugLogger()
			if err != nil {
				l.logger.Error(delayedAuditLoggerTag, err.Error())
				return true, err
			}

			errAuditLogger, err := auditLoggerProvider.ProvideErrorLogger()
			if err != nil {
				l.logger.Error(delayedAuditLoggerTag, err.Error())
				return true, err
			}

			l.debugAuditLogger = debugAuditLogger
			l.errAuditLogger = errAuditLogger
			return false, nil
		})

		unlimitedRetryStrategy := boshretry.NewUnlimitedRetryStrategy(100*time.Millisecond, retryable, l.logger)
		err := unlimitedRetryStrategy.Try()
		if err != nil {
			l.logger.Error(delayedAuditLoggerTag, err.Error())
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
	}()
}

func (l *DelayedAuditLogger) Debug(msg string) {
	l.logger.Debug(delayedAuditLoggerTag, fmt.Sprintf("Logging %s to syslog", msg))

	if len(l.debugLogCh) < 1000 {
		l.debugLogCh <- msg
	} else {
		l.logger.Debug(delayedAuditLoggerTag, fmt.Sprintf("Debug message '%s' not sent to syslog", msg))
	}
}

func (l *DelayedAuditLogger) Err(msg string) {
	l.logger.Debug(delayedAuditLoggerTag, fmt.Sprintf("Logging %s to syslog", msg))

	if len(l.errorLogCh) < 1000 {
		l.errorLogCh <- msg
	} else {
		l.logger.Debug(delayedAuditLoggerTag, fmt.Sprintf("Error message '%s' not sent to syslog", msg))
	}
}
