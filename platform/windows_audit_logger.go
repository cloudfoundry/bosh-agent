// +build windows

package platform

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type DelayedAuditLogger struct{}

func NewDelayedAuditLogger(auditLoggerProvider AuditLoggerProvider, logger boshlog.Logger) *DelayedAuditLogger {
	return &DelayedAuditLogger{}
}

func (w *DelayedAuditLogger) StartLogging() {
}

func (w *DelayedAuditLogger) Debug(msg string) {
}

func (w *DelayedAuditLogger) Err(msg string) {
}
