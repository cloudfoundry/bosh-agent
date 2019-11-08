// +build !windows

package compiler

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

func (c concreteCompiler) moveTmpDir(tmpPath, finalPath string) error {
	err := c.fs.Rename(tmpPath, finalPath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Moving temporary directory %s to final destination %s", tmpPath, finalPath)
	}
	return nil
}
