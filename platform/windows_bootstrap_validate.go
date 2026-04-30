package platform

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	boshwindisk "github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
)

// maxWindowsDiskUniqueIDLen caps the stripped DeviceID; This seemed reasonably large, increse if you find one longer
const maxWindowsDiskUniqueIDLen = 1024

var (
	// Per-label hostname rules (RFC 1123–style): alnum, internal hyphens only; no empty labels or "..".
	ntpHostnameLabelPattern = regexp.MustCompile(`^(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])$`)
	// Ephemeral disk DeviceID (hyphens stripped) is passed as a single-quoted Get-Disk -UniqueId literal.
	// Charset only; max length is maxWindowsDiskUniqueIDLen (RE2 disallows {1,1024}).
	diskIDAllowedCharsPattern = regexp.MustCompile(`^[-A-Za-z0-9._:]+$`)
)

func ValidateNtpServerEntry(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("ntp server entry is empty")
	}
	if net.ParseIP(s) != nil {
		return nil
	}
	if strings.ContainsAny(s, ";`'\"$&|<>\r\n\t\\ ") {
		return errors.New("ntp server entry contains invalid characters")
	}
	if err := validateNtpHostnameLabels(s); err != nil {
		return fmt.Errorf("ntp server entry is not a valid hostname or IP address: %w", err)
	}
	return nil
}

func validateNtpHostnameLabels(host string) error {
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return errors.New("invalid hostname")
	}
	if strings.Contains(host, "..") {
		return errors.New("invalid hostname")
	}
	labels := strings.Split(host, ".")
	for _, lab := range labels {
		if lab == "" || len(lab) > 63 {
			return errors.New("invalid label")
		}
		if !ntpHostnameLabelPattern.MatchString(lab) {
			return errors.New("invalid label")
		}
	}
	return nil
}

func ValidateWindowsDiskUniqueID(stripped string) error {
	if stripped == "" {
		return errors.New("disk DeviceID is empty after removing hyphens")
	}
	if len(stripped) > maxWindowsDiskUniqueIDLen {
		return errors.New("disk DeviceID has invalid characters or exceeds maximum length (after removing hyphens)")
	}
	if !diskIDAllowedCharsPattern.MatchString(stripped) {
		return errors.New("disk DeviceID has invalid characters or exceeds maximum length (after removing hyphens)")
	}
	return nil
}

// ValidateWindowsDiskNumberString returns a canonical decimal string for a non-negative disk index.
func ValidateWindowsDiskNumberString(s string) (string, error) {
	_, canon, err := boshwindisk.ParseDiskNumberString(s)
	return canon, err
}
