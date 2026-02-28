// enable-monit-access is a bosh agent command for BOSH jobs to add monit firewall rules
// to the new nftables-based firewall implemented in the bosh-agent.
//
// Usage:
//
//	bosh-enable-monit-access [<uid>]
//
// When a UID is provided, a UID-based rule is added for that user.
// When no UID is provided, cgroup-based matching is tried first,
// falling back to a UID-based rule for the current user.
//
// This binary serves as a replacement for the complex bash firewall setup logic
// that was previously in job service scripts.
package firewall

import (
	"errors"
	"os"
	"strconv"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func EnableMonitAccess(logger boshlog.Logger, command string, uidArg string) {
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

	logger.Info(command, "Setting up monit firewall rule")

	var uid *uint32
	if uidArg != "" {
		parsed, err := strconv.ParseUint(uidArg, 10, 32)
		if err != nil {
			logger.Error(command, "Invalid UID argument %q: %v. Usage: %s [<uid>]", uidArg, err, command)
			os.Exit(1)
		}
		u := uint32(parsed)
		uid = &u
		logger.Info(command, "UID argument provided: %d", *uid)
	}

	err = mgr.EnableMonitAccess(uid)
	if err != nil {
		logger.Error(command, "Failed to enable monit access: %v", err)
		os.Exit(1)
	}

	logger.Info(command, "Successfully added monit access rule")
}
