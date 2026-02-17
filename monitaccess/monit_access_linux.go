// enable-monit-access is a bosh agent command for BOSH jobs to add monit firewall rules
// to the new nftables-based firewall implemented in the bosh-agent.
//
// Usage:
//
//	bosh-agent enable-monit-access --check    # Check if new firewall is available (exit 0 = yes)
//	bosh-agent enable-monit-access            # Add firewall rule (cgroup preferred, UID fallback)
//
// This binary serves as a replacement for the complex bash firewall setup logic
// that was previously in job service scripts.
package monitaccess

import (
	"fmt"
	"os"
)

func EnableMonitAccess(command string, args []string) {
	// Validate nftables mode: verify if nftables is available
	if len(args) > 1 && args[0] == "--validate-nftables-present" {
		if isNftablesAvailable() {
			os.Exit(0)
		}
		os.Exit(1)
	}

	// 1. Check if jobs chain exists
	chainExists, err := jobsChainExists()
	if err != nil {
		fmt.Printf("enable-monit-access: Failed to check if jobs chain exists: %v\n", err)
		os.Exit(1)
	}

	if !chainExists {
		fmt.Println("enable-monit-access: monit_access_jobs chain not found (old stemcell), skipping")
		os.Exit(0)
	}

	// Setup mode: add firewall rule
	fmt.Println("enable-monit-access: Setting up monit firewall rule")

	// 2. Try cgroup-based rule first (better isolation)
	cgroupPath, err := getCurrentCgroupPath()
	if err == nil && isCgroupAccessible(cgroupPath) {
		inodeID, err := getCgroupInodeID(cgroupPath)
		if err == nil {
			fmt.Printf("enable-monit-access: Using cgroup rule for: %s (inode: %d)\n", cgroupPath, inodeID)

			if err := addCgroupRule(inodeID, cgroupPath); err == nil {
				fmt.Println("enable-monit-access: Successfully added cgroup-based rule")
				os.Exit(0)
			} else {
				fmt.Printf("enable-monit-access: Failed to add cgroup rule: %v\n", err)
			}
		} else {
			fmt.Printf("enable-monit-access: Failed to get cgroup inode ID: %v\n", err)
		}
	} else if err != nil {
		fmt.Printf("enable-monit-access: Could not detect cgroup: %v\n", err)
	}

	// 3. Fallback to UID-based rule
	uid := uint32(os.Getuid())
	fmt.Printf("enable-monit-access: Falling back to UID rule for UID: %d\n", uid)

	if err := addUIDRule(uid); err != nil {
		fmt.Fprintf(os.Stderr, "enable-monit-access: FAILED to add firewall rule: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("enable-monit-access: Successfully added UID-based rule")
}
