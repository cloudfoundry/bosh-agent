// enable-monit-access is a bosh agent command for BOSH jobs to add monit firewall rules
// to the new nftables-based firewall implemented in the bosh-agent.
//
// Usage:
//
//	bosh-agent enable-monit-access # Add firewall rule (cgroup preferred, UID fallback)
//
// This binary serves as a replacement for the complex bash firewall setup logic
// that was previously in job service scripts.
package firewall

import (
	"errors"
	"os"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func EnableMonitAccess(logger boshlog.Logger, command string) {
	logger.UseTags([]boshlog.LogTag{{Name: "monit-access", LogLevel: boshlog.LevelDebug}})

	mgr, err := NewNftablesFirewall(logger)
	if err != nil {
		if errors.Is(err, ErrMonitJobsChainNotFound) {
			logger.Info(command, "monit_access_jobs chain not found (old stemcell), skipping")
			os.Exit(0)
		}
		logger.Error(command, "Failed to create firewall manager: %v", err)
		os.Exit(1)
	}
	defer mgr.Cleanup() //nolint:errcheck

	// Setup mode: add firewall rule
	logger.Info(command, "Setting up monit firewall rule")

	err = mgr.EnableMonitAccess()
	if err != nil {
		logger.Error(command, "Failed to enable monit access: %v", err)
		os.Exit(1)
	}

	logger.Info(command, "Successfully added monit access rule")
}
