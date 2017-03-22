//+build !windows

package system

import (
	"fmt"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"os/user"
	"os"
)

func (fs *osFileSystem) homeDir(username string) (string, error) {
	homeDir, err := fs.runCommand(fmt.Sprintf("echo ~%s", username))
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Shelling out to get user '%s' home directory", username)
	}
	if strings.HasPrefix(homeDir, "~") {
		return "", bosherr.Errorf("Failed to get user '%s' home directory", username)
	}
	return homeDir, nil
}

func (fs *osFileSystem) currentHomeDir() (string, error) {
	return fs.HomeDir("")
}

func (fs *osFileSystem) chown(path, owner string) error {
	parts := strings.Split(owner, ":")

	chownUser, err := user.Lookup(parts[0])
	if err != nil {
		return bosherr.Errorf("Failed to lookup user '%s'", parts[0])
	}

	uid, err := strconv.Atoi(chownUser.Uid)
	if err != nil {
		panic("on POSIX systems uid is always a decimal")
	}

	gid, err := strconv.Atoi(chownUser.Gid)
	if err != nil {
		panic("on POSIX systems uid is always a decimal")
	}

	if len(parts) > 1 {
		var chownGroup *user.Group
		chownGroup, err = user.LookupGroup(parts[1])
		if err != nil {
			return bosherr.Errorf("Failed to lookup group '%s'", parts[1])
		}

		gid, err = strconv.Atoi(chownGroup.Gid)
		if err != nil {
			panic("on POSIX systems uid is always a decimal")
		}
	}

	err = os.Chown(path, uid, gid)
	if err != nil {
		return bosherr.WrapError(err, "Doing Chown")
	}

	return nil
}

func (fs *osFileSystem) symlinkPaths(oldPath, newPath string) (old, new string, err error) {
	return oldPath, newPath, nil
}
