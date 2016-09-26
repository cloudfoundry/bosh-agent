package system

// On Windows user is implemented via syscalls and does not require a C compiler
import "os/user"

import (
	"strings"
	"syscall"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

func (fs *osFileSystem) currentHomeDir() (string, error) {
	t, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer t.Close()
	return t.GetUserProfileDirectory()
}

func (fs *osFileSystem) homeDir(username string) (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	// On Windows, looking up the home directory
	// is only supported for the current user.
	if username != "" && !strings.EqualFold(username, u.Name) {
		return "", bosherr.Errorf("Failed to get user '%s' home directory", username)
	}
	return u.HomeDir, nil
}

func (fs *osFileSystem) chown(path, username string) error {
	return bosherr.WrapError(error(syscall.EWINDOWS), "Chown not supported on Windows")
}
