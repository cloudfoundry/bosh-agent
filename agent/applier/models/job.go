package models

import (
	"os"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type Job struct {
	Name    string
	Version string
	Source  Source

	// Packages that this job depends on; however,
	// currently it will contain packages from all jobs
	Packages []Package
}

func (s Job) BundleName() string {
	return s.Name
}

func (s Job) BundleVersion() string {
	if len(s.Version) == 0 {
		panic("Internal inconsistency: Expected job.Version to be non-empty")
	}

	// Job template is not unique per version because
	// Source contains files with interpolated values
	// which might be different across job versions.
	return s.Version + "-" + s.Source.Sha1.String()
}

type JobDirectoryCreator interface {
	MkdirAll(path string, perm os.FileMode) error
	Chown(path, username string) error
	Chmod(path string, perm os.FileMode) error
	FileExists(path string) bool
}

type JobDirectoryProvider interface {
	JobLogDir(jobName string) string
	JobRunDir(jobName string) string
	JobDir(jobName string) string
}

func (s Job) CreateDirectories(jobDirectoryCreator JobDirectoryCreator, jobDirProvider JobDirectoryProvider) error {
	if len(s.Name) < 1 {
		return bosherr.Error("Job name cannot be empty")
	}

	dirs := []string{
		jobDirProvider.JobLogDir(s.Name),
		jobDirProvider.JobRunDir(s.Name),
		jobDirProvider.JobDir(s.Name),
	}

	for _, dir := range dirs {
		if jobDirectoryCreator.FileExists(dir) {
			continue
		}

		mode := os.FileMode(0770)
		if err := jobDirectoryCreator.MkdirAll(dir, mode); err != nil {
			return bosherr.WrapError(err, "Failed to create dir")
		}

		if err := jobDirectoryCreator.Chmod(dir, mode); err != nil {
			return bosherr.WrapError(err, "Failed to chmod dir")
		}

		if err := jobDirectoryCreator.Chown(dir, "root:vcap"); err != nil {
			return bosherr.WrapError(err, "Failed to chown dir")
		}
	}

	return nil
}
