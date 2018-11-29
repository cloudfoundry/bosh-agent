package bundlecollection

import (
	"time"
)

const BundleSetupTimeout = 2 * time.Minute

func (b FileBundle) Uninstall() error {
	b.logger.Debug(fileBundleLogTag, "Uninstalling %v", b)

	// RemoveAll MUST be the last possibly-failing operation
	// because IsInstalled() relies on installPath presence.

	var err error
	startTime := b.timeProvider.Now()

	for b.timeProvider.Since(startTime) < BundleSetupTimeout {
		err = b.fs.RemoveAll(b.installPath)
		if err == nil {
			return nil
		}
		b.timeProvider.Sleep(time.Second * 5)
	}

	return err
}
