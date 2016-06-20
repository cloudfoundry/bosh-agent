package fs

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func absPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, path), nil
}

func winPath(path string) (string, error) {
	p, err := absPath(path)
	if err != nil {
		return "", err
	}
	switch n := len(p); {
	case n >= syscall.MAX_LONG_PATH:
		err := &os.PathError{
			Op:   "fs: Path",
			Path: path,
			Err:  errors.New("path length exceeds MAX_LONG_PATH"),
		}
		return "", err
	case n >= syscall.MAX_PATH:
		return `\\?\` + p, nil
	default:
		return path, nil
	}
}

func newPathError(op, path string, err error) error {
	return &os.PathError{
		Op:   "fs: " + op,
		Path: path,
		Err:  err,
	}
}

func newLinkError(op, oldname, newname string, err error) error {
	return &os.LinkError{
		Op:  "fs: " + op,
		Old: oldname,
		New: newname,
		Err: err,
	}
}

func chdir(dir string) error {
	p, err := winPath(dir)
	if err != nil {
		return newPathError("chdir", dir, err)
	}
	return os.Chdir(p)
}

func chmod(name string, mode os.FileMode) error {
	p, err := winPath(name)
	if err != nil {
		return newPathError("chmod", name, err)
	}
	return os.Chmod(p, mode)
}

func chown(name string, uid, gid int) error {
	p, err := winPath(name)
	if err != nil {
		return newPathError("chown", name, err)
	}
	return os.Chown(p, uid, gid)
}

func chtimes(name string, atime time.Time, mtime time.Time) error {
	p, err := winPath(name)
	if err != nil {
		return newPathError("chtimes", name, err)
	}
	return os.Chtimes(p, atime, mtime)
}

func lchown(name string, uid, gid int) error {
	p, err := winPath(name)
	if err != nil {
		return newPathError("lchown", name, err)
	}
	return os.Lchown(p, uid, gid)
}

func link(oldname, newname string) error {
	op, err := winPath(oldname)
	if err != nil {
		return newLinkError("link", oldname, newname, err)
	}
	np, err := winPath(newname)
	if err != nil {
		return newLinkError("link", oldname, newname, err)
	}
	return os.Link(op, np)
}

func mkdir(name string, perm os.FileMode) error {
	p, err := winPath(name)
	if err != nil {
		return newPathError("mkdir", name, err)
	}
	return os.Mkdir(p, perm)
}

func mkdirall(path string, perm os.FileMode) error {
	p, err := winPath(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(p, perm)
}

func readlink(name string) (string, error) {
	p, err := winPath(name)
	if err != nil {
		return "", newPathError("readlink", name, err)
	}
	return os.Readlink(p)
}

func remove(name string) error {
	p, err := winPath(name)
	if err != nil {
		return newPathError("remove", name, err)
	}
	return os.Remove(p)
}

func removeall(path string) error {
	p, err := winPath(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(p)
}

func rename(oldpath, newpath string) error {
	op, err := winPath(oldpath)
	if err != nil {
		return newLinkError("rename", oldpath, newpath, err)
	}
	np, err := winPath(newpath)
	if err != nil {
		return newLinkError("rename", oldpath, newpath, err)
	}
	return os.Rename(op, np)
}

func symlink(oldname, newname string) error {
	op, err := winPath(oldname)
	if err != nil {
		return newLinkError("symlink", oldname, newname, err)
	}
	np, err := winPath(newname)
	if err != nil {
		return newLinkError("symlink", oldname, newname, err)
	}
	return os.Symlink(op, np)
}

func create(name string) (*os.File, error) {
	p, err := winPath(name)
	if err != nil {
		return nil, newPathError("create", name, err)
	}
	return os.Create(p)
}

func newfile(fd uintptr, name string) *os.File {
	p, err := winPath(name)
	if err != nil {
		return os.NewFile(fd, name)
	}
	return os.NewFile(fd, p)
}

func open(name string) (*os.File, error) {
	p, err := winPath(name)
	if err != nil {
		return nil, newPathError("open", name, err)
	}
	return os.Open(p)
}

func openfile(name string, flag int, perm os.FileMode) (*os.File, error) {
	p, err := winPath(name)
	if err != nil {
		return nil, newPathError("openfile", name, err)
	}
	return os.OpenFile(p, flag, perm)
}

func lstat(name string) (os.FileInfo, error) {
	p, err := winPath(name)
	if err != nil {
		return nil, newPathError("lstat", name, err)
	}
	return os.Lstat(p)
}

func stat(name string) (os.FileInfo, error) {
	p, err := winPath(name)
	if err != nil {
		return nil, newPathError("stat", name, err)
	}
	return os.Stat(p)
}
