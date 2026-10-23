//go:build !windows

package platform

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func setupSSHFiles(home, username string, keys []string) error {
	account, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("looking up SSH account: %w", err)
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return err
	}
	// Anchor every subsequent operation to directory descriptors. Renaming a
	// user-owned path or replacing it with a symlink cannot redirect root writes.
	homeFD, err := unix.Open(home, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("opening SSH home directory: %w", err)
	}
	defer func() {
		_ = unix.Close(homeFD) //nolint:errcheck
	}()
	if err := unix.Mkdirat(homeFD, ".ssh", 0700); err != nil && err != unix.EEXIST {
		return fmt.Errorf("creating SSH directory: %w", err)
	}
	sshFD, err := unix.Openat(homeFD, ".ssh", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("opening SSH directory: %w", err)
	}
	defer func() {
		_ = unix.Close(sshFD) //nolint:errcheck
	}()
	if err := unix.Fchown(sshFD, uid, gid); err != nil {
		return fmt.Errorf("owning SSH directory: %w", err)
	}
	if err := unix.Fchmod(sshFD, 0700); err != nil {
		return fmt.Errorf("setting SSH directory mode: %w", err)
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	temporary := ".authorized_keys-" + hex.EncodeToString(random)
	fd, err := unix.Openat(sshFD, temporary, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return fmt.Errorf("creating SSH keys file: %w", err)
	}
	defer func() {
		_ = unix.Unlinkat(sshFD, temporary, 0) //nolint:errcheck
	}()
	file := os.NewFile(uintptr(fd), temporary)
	defer func() {
		_ = file.Close() //nolint:errcheck
	}()
	if err := file.Chown(uid, gid); err != nil {
		return err
	}
	if err := file.Chmod(0600); err != nil {
		return err
	}
	content := strings.Join(keys, "\n")
	n, err := io.WriteString(file, content)
	if err != nil {
		return err
	}
	if n != len(content) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return err
	}
	// Replacing an existing symlink/hardlink entry never opens its target.
	if err := unix.Renameat(sshFD, temporary, sshFD, "authorized_keys"); err != nil {
		return fmt.Errorf("publishing SSH keys file: %w", err)
	}
	return nil
}
