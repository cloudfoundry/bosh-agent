// +build !windows

package platform

import "errors"

var ErrNotImplemented = errors.New("not implemented")

func createUserProfile(username string) error {
	return ErrNotImplemented
}
