// +build !windows

package compiler

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"time"
)

const CompileTimeout = 2 * time.Minute

func (c concreteCompiler) moveTmpDir(tmpPath, finalPath string) error {
	startTime := c.timeProvider.Now()
	var err error
	for timeoutExceeded := false; !timeoutExceeded; timeoutExceeded = c.timeProvider.Since(startTime) > CompileTimeout {
		err = c.fs.Rename(tmpPath, finalPath)
		if err == nil {
			break
		}
	}
	if err != nil {
		return bosherr.WrapErrorf(err, "Moving temporary directory %s to final destination %s", tmpPath, finalPath)
	}
	return nil
}
