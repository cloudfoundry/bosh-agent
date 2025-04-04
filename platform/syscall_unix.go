//go:build !windows
// +build !windows

package platform

import (
	"errors"
)

var ErrNotImplemented = errors.New("not implemented")

func userExists(_ string) bool {
	return false
}

func createUserProfile(username string) error {
	return ErrNotImplemented
}

func deleteLocalUser(username string) error {
	return ErrNotImplemented
}

func userHomeDirectory(username string) (string, error) {
	return "", ErrNotImplemented
}

func localAccountNames() ([]string, error) {
	return nil, ErrNotImplemented
}

func sshEnabled() error {
	return ErrNotImplemented
}

func setupRuntimeConfiguration() error {
	return ErrNotImplemented
}

func setRandomPassword(username string) error {
	return ErrNotImplemented
}
