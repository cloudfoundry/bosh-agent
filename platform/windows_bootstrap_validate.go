package platform

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	boshwindisk "github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
)

var (
	// Per-label hostname rules (RFC 1123–style): alnum, internal hyphens only; no empty labels or "..".
	ntpHostnameLabelPattern = regexp.MustCompile(`^(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])$`)
	diskUniqueHexPattern    = regexp.MustCompile(`^[0-9A-Fa-f]{8,128}$`)
)

// ValidateNtpServerEntry returns nil if s is a valid IPv4/IPv6 literal or a conservative hostname for NTP.
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

// ValidateWindowsDiskUniqueIDHex validates the DeviceID string after hyphens are removed (UniqueId form for Get-Disk).
func ValidateWindowsDiskUniqueIDHex(stripped string) error {
	if !diskUniqueHexPattern.MatchString(stripped) {
		return errors.New("disk DeviceID must be hexadecimal (after removing hyphens)")
	}
	return nil
}

// ValidateWindowsDiskNumberString returns a canonical decimal string for a non-negative disk index.
func ValidateWindowsDiskNumberString(s string) (string, error) {
	_, canon, err := boshwindisk.ParseDiskNumberString(s)
	return canon, err
}
