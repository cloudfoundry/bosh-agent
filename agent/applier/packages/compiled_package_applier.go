package packages

import (
	bc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	models "github.com/cloudfoundry/bosh-agent/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const logTag = "compiledPackageApplier"

type compiledPackageApplier struct {
	packagesBc bc.BundleCollection

	// KeepOnly will permanently uninstall packages when operating as owner
	packagesBcOwner bool

	blobstore blobstore_delegator.BlobstoreDelegator
	fs        boshsys.FileSystem
	logger    boshlog.Logger
}

func NewCompiledPackageApplier(
	packagesBc bc.BundleCollection,
	packagesBcOwner bool,
	blobstore blobstore_delegator.BlobstoreDelegator,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) Applier {
	return &compiledPackageApplier{
		packagesBc:      packagesBc,
		packagesBcOwner: packagesBcOwner,
		blobstore:       blobstore,
		fs:              fs,
		logger:          logger,
	}
}

func (s compiledPackageApplier) Prepare(pkg models.Package) error {
	s.logger.Debug(logTag, "Preparing package %v", pkg)

	pkgBundle, err := s.packagesBc.Get(pkg)
	if err != nil {
		return bosherr.WrapError(err, "Getting package bundle")
	}

	pkgInstalled, err := pkgBundle.IsInstalled()
	if err != nil {
		return bosherr.WrapError(err, "Checking if package is installed")
	}

	if !pkgInstalled {
		err := s.downloadAndInstall(pkg, pkgBundle)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s compiledPackageApplier) Apply(pkg models.Package) error {
	s.logger.Debug(logTag, "Applying package %v", pkg)

	err := s.Prepare(pkg)
	if err != nil {
		return err
	}

	pkgBundle, err := s.packagesBc.Get(pkg)
	if err != nil {
		return bosherr.WrapError(err, "Getting package bundle")
	}

	_, err = pkgBundle.Enable()
	if err != nil {
		return bosherr.WrapError(err, "Enabling package")
	}

	return nil
}

func (s *compiledPackageApplier) downloadAndInstall(pkg models.Package, pkgBundle bc.Bundle) error {
	file, err := s.blobstore.Get(pkg.Source.Sha1, pkg.Source.SignedURL, pkg.Source.BlobstoreID, pkg.Source.BlobstoreHeaders)
	if err != nil {
		return bosherr.WrapError(err, "Fetching package blob")
	}

	defer func() {
		if err = s.blobstore.CleanUp("", file); err != nil {
			s.logger.Warn(logTag, "Failed to clean up blobstore blob: %s", err.Error())
		}
	}()

	_, err = pkgBundle.Install(file, "")
	if err != nil {
		return bosherr.WrapError(err, "Installing package directory")
	}

	return nil
}

func (s *compiledPackageApplier) KeepOnly(pkgs []models.Package) error {
	s.logger.Debug(logTag, "Keeping only packages %v", pkgs)

	installedBundles, err := s.packagesBc.List()
	if err != nil {
		return bosherr.WrapError(err, "Retrieving installed bundles")
	}

	for _, installedBundle := range installedBundles {
		var shouldKeep bool

		for _, pkg := range pkgs {
			pkgBundle, err := s.packagesBc.Get(pkg)
			if err != nil {
				return bosherr.WrapError(err, "Getting package bundle")
			}

			if pkgBundle == installedBundle {
				shouldKeep = true
				break
			}
		}

		if !shouldKeep {
			err = installedBundle.Disable()
			if err != nil {
				return bosherr.WrapError(err, "Disabling package bundle")
			}

			if s.packagesBcOwner {
				// If we uninstall the bundle first, and the disable failed (leaving the symlink),
				// then the next time bundle collection will not include bundle in its list
				// which means that symlink will never be deleted.
				err = installedBundle.Uninstall()
				if err != nil {
					return bosherr.WrapError(err, "Uninstalling package bundle")
				}
			}
		}
	}

	return nil
}
