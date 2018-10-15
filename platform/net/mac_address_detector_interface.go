package net

type MACAddressDetector interface {
	DetectMacAddresses() (map[string]string, error)
}
