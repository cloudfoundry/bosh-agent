package compiler

import (
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

const CompileTimeout = 2 * time.Minute

func (c concreteCompiler) moveTmpDir(tmpPath, finalPath string) error {
	var err error
	startTime := c.timeProvider.Now()
	for timeoutExceeded := false; !timeoutExceeded; timeoutExceeded = c.timeProvider.Since(startTime) > CompileTimeout {
		err = c.fs.Rename(tmpPath, finalPath)
		if err == nil {
			break
		}
		c.timeProvider.Sleep(5 * time.Second)
	}
	if err != nil {
		return bosherr.WrapErrorf(err, "Moving temporary directory %s to final destination %s", tmpPath, finalPath)
	}
	return nil
}
