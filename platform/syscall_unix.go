// +build !windows

package platform

import "errors"

var ErrNotImplemented = errors.New("not implemented")

func CreateUserProfile(username string) error {
	return ErrNotImplemented
}
