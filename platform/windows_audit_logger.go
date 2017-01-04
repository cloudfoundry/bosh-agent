// +build windows

package platform

type WindowsAuditLogger struct{}

func NewDelayedAuditLogger() AuditLogger {
	return &WindowsAuditLogger{}
}

func (w *WindowsAuditLogger) Debug(msg string) error {
	return nil
}

func (w *WindowsAuditLogger) Err(msg string) error {
	return nil
}
