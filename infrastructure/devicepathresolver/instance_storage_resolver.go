package devicepathresolver

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

// InstanceStorageResolver discovers instance storage devices, filtering out
// IaaS-managed volumes like EBS, persistent disks, etc.
type InstanceStorageResolver interface {
	// DiscoverInstanceStorage takes a list of expected ephemeral disks and returns
	// the actual device paths for instance storage, excluding IaaS-managed volumes
	DiscoverInstanceStorage(devices []boshsettings.DiskSettings) ([]string, error)
}
