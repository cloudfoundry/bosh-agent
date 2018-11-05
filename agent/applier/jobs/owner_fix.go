package jobs

import (
	"fmt"
	"os"
	gopath "path"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// FixPermissions changes the permissions of the rendered job templates to be
// consistent for every job. The path is the root of the job templates
// directory e.g. /var/vcap/data/jobs/JOBNAME.
func FixPermissions(fs boshsys.FileSystem, path string, user string, group string) error {
	ug := fmt.Sprintf("%s:%s", user, group)
	binPath := gopath.Join(path, "bin") + "/"

	err := fs.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if err := fs.Chown(path, ug); err != nil {
			return bosherr.WrapError(err, "Failed to chown dir")
		}

		if info.IsDir() {
			return fs.Chmod(path, os.FileMode(0750))
		}

		// If the file is in /var/vcap/jobs/JOB/bin.
		if strings.HasPrefix(path, binPath) {
			return fs.Chmod(path, os.FileMode(0750))
		}

		return fs.Chmod(path, os.FileMode(0640))
	})

	if err != nil {
		return bosherr.WrapError(err, "Correcting file permissions")
	}

	return nil
}
