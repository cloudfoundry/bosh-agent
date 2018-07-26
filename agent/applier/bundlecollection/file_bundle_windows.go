package bundlecollection

import "time"

const TIMEOUT_IN_MINUTES = 2

func (b FileBundle) Uninstall() error {
	b.logger.Debug(fileBundleLogTag, "Uninstalling %v", b)

	timeoutEnd := time.Now().UTC().Unix() + TIMEOUT_IN_MINUTES * 60

	var err error

	// Windows sometimes holds execution locks for a long time after
	// an executable has apparently finished so we may need to retry
	for timeoutEnd > time.Now().UTC().Unix() {
		// RemoveAll MUST be the last possibly-failing operation
		// because IsInstalled() relies on installPath presence.
		err = b.fs.RemoveAll(b.installPath)
		if err == nil {
			return nil
		}

		time.Sleep(time.Second * 5)
	}

	return err
}