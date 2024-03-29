package utils

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type TarWalker struct {
	tw   *tar.Writer
	root string
}

func trimPathPrefix(s, prefix string) string {
	return strings.TrimLeft(strings.TrimPrefix(s, prefix), "/")
}

func (t *TarWalker) Walk(path string, fi os.FileInfo, _ error) error {
	// If path == root we are adding only the contents of the directory
	if path == t.root {
		return nil
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	if fi.IsDir() {
		hdr.Name = trimPathPrefix(path+string(filepath.Separator), t.root)
	} else {
		hdr.Name = trimPathPrefix(path, t.root)
	}
	if hdr.Name == "" {
		return fmt.Errorf("invalid name: %q for root: %q", hdr.Name, t.root)
	}
	if err := t.tw.WriteHeader(hdr); err != nil {
		return err
	}
	if !fi.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(t.tw, f); err != nil {
			f.Close() //nolint:errcheck
			return err
		}
		f.Close() //nolint:errcheck
	}
	return nil
}

// TarballDirectory - rootdir is equivalent to tar -C 'rootdir'
func TarballDirectory(dirname, rootdir, tarname string) (string, error) {
	f, err := os.OpenFile(tarname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return "", err
	}
	h := sha1.New()
	gw := gzip.NewWriter(io.MultiWriter(f, h))
	tw := tar.NewWriter(gw)

	w := TarWalker{
		tw:   tw,
		root: rootdir,
	}
	if err := filepath.Walk(dirname, w.Walk); err != nil {
		return "", err
	}

	if err := tw.Close(); err != nil {
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
