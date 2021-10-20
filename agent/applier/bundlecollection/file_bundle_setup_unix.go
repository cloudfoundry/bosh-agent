//go:build !windows
// +build !windows

package bundlecollection

func (b FileBundle) Uninstall() error {
	b.logger.Debug(fileBundleLogTag, "Uninstalling %v", b)

	// RemoveAll MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.
	return b.fs.RemoveAll(b.installPath)
}
