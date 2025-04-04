package jobs

import (
	"fmt"
	"path"
	"strings"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/packages"
	"github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider/blobstore_delegator"
	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshbc "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
)

const logTag = "renderedJobApplier"

type FixPermissionsFunc func(boshsys.FileSystem, string, string, string) error

type renderedJobApplier struct {
	blobstore              blobstore_delegator.BlobstoreDelegator
	dirProvider            directories.Provider
	fixPermissions         FixPermissionsFunc
	fs                     boshsys.FileSystem
	jobSupervisor          boshjobsuper.JobSupervisor
	jobsBc                 boshbc.BundleCollection
	logger                 boshlog.Logger
	packageApplierProvider packages.ApplierProvider
}

func NewRenderedJobApplier(
	blobstore blobstore_delegator.BlobstoreDelegator,
	dirProvider directories.Provider,
	jobsBc boshbc.BundleCollection,
	jobSupervisor boshjobsuper.JobSupervisor,
	packageApplierProvider packages.ApplierProvider,
	fixPermissions FixPermissionsFunc,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) Applier {
	return &renderedJobApplier{
		blobstore:              blobstore,
		dirProvider:            dirProvider,
		fixPermissions:         fixPermissions,
		fs:                     fs,
		jobSupervisor:          jobSupervisor,
		jobsBc:                 jobsBc,
		logger:                 logger,
		packageApplierProvider: packageApplierProvider,
	}
}

func (s renderedJobApplier) Prepare(job models.Job) error {
	s.logger.Debug(logTag, "Preparing job %v", job)

	jobBundle, err := s.jobsBc.Get(job)
	if err != nil {
		return bosherr.WrapError(err, "Getting job bundle")
	}

	jobInstalled, err := jobBundle.IsInstalled()
	if err != nil {
		return bosherr.WrapError(err, "Checking if job is installed")
	}

	if !jobInstalled {
		err = s.downloadAndInstall(job, jobBundle)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *renderedJobApplier) Apply(job models.Job) error {
	s.logger.Debug(logTag, "Applying job %v", job)

	err := s.Prepare(job)
	if err != nil {
		return bosherr.WrapError(err, "Preparing job")
	}

	if err := job.CreateDirectories(s.fs, s.dirProvider); err != nil {
		return bosherr.WrapErrorf(err, "Creating directories for job %s", job.Name)
	}

	jobBundle, err := s.jobsBc.Get(job)
	if err != nil {
		return bosherr.WrapError(err, "Getting job bundle")
	}

	_, err = jobBundle.Enable()
	if err != nil {
		return bosherr.WrapError(err, "Enabling job")
	}

	return s.applyPackages(job)
}

func (s *renderedJobApplier) downloadAndInstall(job models.Job, jobBundle boshbc.Bundle) error {
	file, err := s.blobstore.Get(boshcrypto.MustNewMultipleDigest(job.Source.Sha1), job.Source.SignedURL, job.Source.BlobstoreID, job.Source.BlobstoreHeaders)
	if err != nil {
		return bosherr.WrapError(err, "Getting job source from blobstore")
	}

	defer func() {
		if err = s.blobstore.CleanUp("", file); err != nil {
			s.logger.Warn(logTag, "Failed to clean up blobstore blob: %s", err.Error())
		}
	}()

	_, err = jobBundle.Install(file, job.Source.PathInArchive)
	if err != nil {
		return bosherr.WrapError(err, "Installing job bundle")
	}

	installPath, err := jobBundle.GetInstallPath()
	if err != nil {
		return bosherr.WrapError(err, "Getting the install path")
	}

	err = s.fixPermissions(s.fs, installPath, "root", "vcap")
	if err != nil {
		return bosherr.WrapError(err, "Fixing job bundle permissions")
	}

	return nil
}

// applyPackages keeps job specific packages directory up-to-date with installed packages.
// (e.g. /var/vcap/jobs/job-a/packages/pkg-a has symlinks to /var/vcap/packages/pkg-a)
func (s *renderedJobApplier) applyPackages(job models.Job) error {
	packageApplier := s.packageApplierProvider.JobSpecific(job.Name)

	for _, pkg := range job.Packages {
		err := packageApplier.Apply(pkg)
		if err != nil {
			return bosherr.WrapErrorf(err, "Applying package %s for job %s", pkg.Name, job.Name)
		}
	}

	err := packageApplier.KeepOnly(job.Packages)
	if err != nil {
		return bosherr.WrapErrorf(err, "Keeping only needed packages for job %s", job.Name)
	}

	return nil
}

func (s *renderedJobApplier) Configure(job models.Job, jobIndex int) (err error) {
	s.logger.Debug(logTag, "Configuring job %v with index %d", job, jobIndex)

	jobBundle, err := s.jobsBc.Get(job)
	if err != nil {
		err = bosherr.WrapError(err, "Getting job bundle")
		return
	}

	jobDir, err := jobBundle.GetInstallPath()
	if err != nil {
		err = bosherr.WrapError(err, "Looking up job directory")
		return
	}

	monitFilePath := path.Join(jobDir, "monit")
	if s.fs.FileExists(monitFilePath) {
		err = s.jobSupervisor.AddJob(job.Name, jobIndex, monitFilePath)
		if err != nil {
			err = bosherr.WrapError(err, "Adding monit configuration")
			return
		}
	}

	monitFilePaths, err := s.fs.Glob(path.Join(jobDir, "*.monit"))
	if err != nil {
		err = bosherr.WrapError(err, "Looking for additional monit files")
		return
	}

	for _, monitFilePath := range monitFilePaths {
		label := strings.Replace(path.Base(monitFilePath), ".monit", "", 1)
		subJobName := fmt.Sprintf("%s_%s", job.Name, label)

		err = s.jobSupervisor.AddJob(subJobName, jobIndex, monitFilePath)
		if err != nil {
			err = bosherr.WrapErrorf(err, "Adding additional monit configuration %s", label)
			return
		}
	}

	return nil
}

func (s *renderedJobApplier) KeepOnly(jobs []models.Job) error {
	s.logger.Debug(logTag, "Keeping only jobs %v", jobs)

	installedBundles, err := s.jobsBc.List()
	if err != nil {
		return bosherr.WrapError(err, "Retrieving installed bundles")
	}

	for _, installedBundle := range installedBundles {
		var shouldKeep bool

		for _, job := range jobs {
			jobBundle, err := s.jobsBc.Get(job)
			if err != nil {
				return bosherr.WrapError(err, "Getting job bundle")
			}

			if jobBundle == installedBundle {
				shouldKeep = true
				break
			}
		}

		if !shouldKeep {
			err = installedBundle.Disable()
			if err != nil {
				return bosherr.WrapError(err, "Disabling job bundle")
			}

			// If we uninstall the bundle first, and the disable failed (leaving the symlink),
			// then the next time bundle collection will not include bundle in its list
			// which means that symlink will never be deleted.
			err = installedBundle.Uninstall()
			if err != nil {
				return bosherr.WrapError(err, "Uninstalling job bundle")
			}
		}
	}

	return nil
}

func (s *renderedJobApplier) DeleteSourceBlobs(jobs []models.Job) error {
	deletedBlobs := map[string]bool{}

	for _, job := range jobs {
		if _, ok := deletedBlobs[job.Source.BlobstoreID]; ok {
			continue
		}

		err := s.blobstore.Delete("", job.Source.BlobstoreID)
		if err != nil {
			return err
		}

		deletedBlobs[job.Source.BlobstoreID] = true
	}

	return nil
}
