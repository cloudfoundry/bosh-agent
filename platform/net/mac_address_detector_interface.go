package net

//go:generate counterfeiter . MACAddressDetector

type MACAddressDetector interface {
	DetectMacAddresses() (map[string]string, error)
}
