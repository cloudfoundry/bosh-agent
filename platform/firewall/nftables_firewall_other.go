//go:build !linux

package firewall

import (
	"errors"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

// NewNftablesFirewall returns an error on non-Linux platforms
func NewNftablesFirewall(logger boshlog.Logger) (Manager, error) {
	return nil, errors.New("nftables firewall is only supported on Linux")
}
