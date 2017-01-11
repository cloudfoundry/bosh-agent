//+build !windows

package platform

import (
	"log"
	"log/syslog"
)

type linuxAuditLoggerProvider struct{}

func NewAuditLoggerProvider() AuditLoggerProvider {
	return &linuxAuditLoggerProvider{}
}

func (p *linuxAuditLoggerProvider) ProvideDebugLogger() (*log.Logger, error) {
	return syslog.NewLogger(syslog.LOG_DEBUG, log.LstdFlags)
}

func (p *linuxAuditLoggerProvider) ProvideErrorLogger() (*log.Logger, error) {
	return syslog.NewLogger(syslog.LOG_ERR, log.LstdFlags)
}
