//go:build !linux

package firewall

import (
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

// noopFirewall is a no-op firewall manager for non-Linux platforms
type noopFirewall struct{}

// NewNftablesFirewall returns a no-op firewall manager on non-Linux platforms
func NewNftablesFirewall(logger boshlog.Logger) (Manager, error) {
	return &noopFirewall{}, nil
}

func (f *noopFirewall) SetupAgentRules(mbusURL string, enableNATSFirewall bool) error {
	return nil
}

func (f *noopFirewall) AllowService(service Service, callerPID int) error {
	return nil
}

func (f *noopFirewall) Cleanup() error {
	return nil
}
