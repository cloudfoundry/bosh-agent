package commands

import (
	"os"
	"path/filepath"

	"github.com/cloudfoundry/gofileutils/glob"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

const cpCopierLogTag = "cpCopier"

type cpCopier struct {
	fs        boshsys.FileSystem
	cmdRunner boshsys.CmdRunner
	logger    boshlog.Logger
}

func NewCpCopier(
	cmdRunner boshsys.CmdRunner,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) Copier {
	return cpCopier{fs: fs, cmdRunner: cmdRunner, logger: logger}
}

func (c cpCopier) FilteredCopyToTemp(dir string, filters []string) (string, error) {
	tempDir, err := c.fs.TempDir("bosh-platform-commands-cpCopier-FilteredCopyToTemp")
	if err != nil {
		return "", bosherr.WrapError(err, "Creating temporary directory")
	}

	dirGlob := glob.NewDir(dir)
	filesToCopy, err := dirGlob.Glob(filters...)
	if err != nil {
		c.CleanUp(tempDir)
		return "", bosherr.WrapError(err, "Finding files matching filters")
	}

	for _, relativePath := range filesToCopy {
		src := filepath.Join(dir, relativePath)
		dst := filepath.Join(tempDir, relativePath)

		fileInfo, err := os.Stat(src)
		if err != nil {
			return "", bosherr.WrapErrorf(err, "Getting file info for '%s'", src)
		}

		if fileInfo.IsDir() {
			err = c.cpDir(src, dst, tempDir)
		} else {
			err = c.cpFile(src, dst, tempDir)
		}

		if err != nil {
			c.CleanUp(tempDir)
			return "", err
		}
	}

	err = c.fs.Chmod(tempDir, os.FileMode(0755))
	if err != nil {
		c.CleanUp(tempDir)
		return "", bosherr.WrapError(err, "Fixing permissions on temp dir")
	}

	return tempDir, nil
}

func (c cpCopier) CleanUp(tempDir string) {
	err := c.fs.RemoveAll(tempDir)
	if err != nil {
		c.logger.Error(cpCopierLogTag, "Failed to clean up temporary directory %s: %#v", tempDir, err)
	}
}

// Because of globs, when we're copying a directory it's possible that directory already
// exists in the temp dir. This function copies the directory's contents to make sure
// it doesn't create a duplicate, nested directory structure
func (c cpCopier) cpDir(src, dst, tempDir string) error {
	err := c.fs.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making destination directory '%s' for '%s'", dst, src)
	}

	srcWithTrailingSlash := src + string(os.PathSeparator)

	return c.cpRP(srcWithTrailingSlash, dst, tempDir)
}

func (c cpCopier) cpFile(src, dst, tempDir string) error {
	containingDir := filepath.Dir(dst)
	err := c.fs.MkdirAll(containingDir, os.ModePerm)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making destination directory '%s' for '%s'", containingDir, src)
	}

	return c.cpRP(src, dst, tempDir)
}

func (c cpCopier) cpRP(src, dst, tempDir string) error {
	// Golang does not have a way of copying files and preserving file info...
	_, _, _, err := c.cmdRunner.RunCommand("cp", "-Rp", src, dst)
	if err != nil {
		c.CleanUp(tempDir)
		return bosherr.WrapError(err, "Shelling out to cp")
	}

	return nil
}
