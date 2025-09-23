package compiler

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/clock"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshbc "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
	boshmodels "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/packages"
	boshcmdrunner "github.com/cloudfoundry/bosh-agent/v2/agent/cmdrunner"
	"github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider/blobstore_delegator"
)

const PackagingScriptName = "packaging"

type CompileDirProvider interface {
	CompileDir() string
}

type concreteCompiler struct {
	compressor         boshcmd.Compressor
	blobstore          blobstore_delegator.BlobstoreDelegator
	fs                 boshsys.FileSystem
	runner             boshcmdrunner.CmdRunner
	compileDirProvider CompileDirProvider
	packageApplier     packages.Applier
	packagesBc         boshbc.BundleCollection
	timeProvider       clock.Clock
}

func NewConcreteCompiler(
	compressor boshcmd.Compressor,
	blobstore blobstore_delegator.BlobstoreDelegator,
	fs boshsys.FileSystem,
	runner boshcmdrunner.CmdRunner,
	compileDirProvider CompileDirProvider,
	packageApplier packages.Applier,
	packagesBc boshbc.BundleCollection,
	timeProvider clock.Clock,
) Compiler {
	return concreteCompiler{
		compressor:         compressor,
		blobstore:          blobstore,
		fs:                 fs,
		runner:             runner,
		compileDirProvider: compileDirProvider,
		packageApplier:     packageApplier,
		packagesBc:         packagesBc,
		timeProvider:       timeProvider,
	}
}

func (c concreteCompiler) Compile(pkg Package, deps []boshmodels.Package) (blobID string, digest boshcrypto.Digest, err error) {
	err = c.packageApplier.KeepOnly([]boshmodels.Package{})
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Removing packages")
	}

	for _, dep := range deps {
		err := c.packageApplier.Apply(dep)
		if err != nil {
			return "", nil, bosherr.WrapErrorf(err, "Installing dependent package: '%s'", dep.Name)
		}
	}

	compilePath := path.Join(c.compileDirProvider.CompileDir(), pkg.Name)

	err = c.fetchAndUncompress(pkg, compilePath)
	if err != nil {
		return "", nil, bosherr.WrapErrorf(err, "Fetching package %s", pkg.Name)
	}

	defer func() {
		e := c.fs.RemoveAll(compilePath)
		if e != nil && err == nil {
			err = e
		}
	}()

	compiledPkg := boshmodels.LocalPackage{
		Name:    pkg.Name,
		Version: pkg.Version,
	}

	compiledPkgBundle, err := c.packagesBc.Get(compiledPkg)
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Getting bundle for new package")
	}

	installPath, err := compiledPkgBundle.InstallWithoutContents()
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Setting up new package bundle")
	}

	enablePath, err := compiledPkgBundle.Enable()
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Enabling new package bundle")
	}

	scriptPath := path.Join(compilePath, PackagingScriptName)

	if c.fs.FileExists(scriptPath) {
		if err := c.runPackagingCommand(compilePath, enablePath, pkg); err != nil {
			return "", nil, bosherr.WrapError(err, "Running packaging script")
		}
	}

	tmpPackageTar, err := c.compressor.CompressFilesInDir(installPath, boshcmd.CompressorOptions{})
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Compressing compiled package")
	}

	defer func() {
		_ = c.compressor.CleanUp(tmpPackageTar) //nolint:errcheck
	}()

	uploadedBlobID, digest, err := c.blobstore.Write(pkg.UploadSignedURL, tmpPackageTar, pkg.BlobstoreHeaders)
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Uploading compiled package")
	}

	err = compiledPkgBundle.Disable()
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Disabling compiled package")
	}

	err = compiledPkgBundle.Uninstall()
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Uninstalling compiled package")
	}

	err = c.packageApplier.KeepOnly([]boshmodels.Package{})
	if err != nil {
		return "", nil, bosherr.WrapError(err, "Removing packages")
	}

	return uploadedBlobID, digest, nil
}

func (c concreteCompiler) fetchAndUncompress(pkg Package, targetDir string) error {
	if pkg.BlobstoreID == "" && pkg.PackageGetSignedURL == "" {
		return bosherr.Error(fmt.Sprintf("No blobstore reference for package '%s'", pkg.Name))
	}

	depFilePath, err := c.blobstore.Get(pkg.Sha1, pkg.PackageGetSignedURL, pkg.BlobstoreID, pkg.BlobstoreHeaders)
	if err != nil {
		return bosherr.WrapErrorf(err, "Fetching package blob %s", pkg.BlobstoreID)
	}

	err = c.atomicDecompress(depFilePath, targetDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Uncompressing package %s", pkg.Name)
	}

	return nil
}

func (c concreteCompiler) atomicDecompress(archivePath string, finalDir string) error {
	tmpInstallPath := finalDir + "-bosh-agent-unpack"

	{
		err := c.fs.RemoveAll(finalDir)
		if err != nil {
			return bosherr.WrapErrorf(err, "Removing install path %s", finalDir)
		}

		err = c.fs.MkdirAll(finalDir, os.FileMode(0755))
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating install path %s", finalDir)
		}
	}

	{
		err := c.fs.RemoveAll(tmpInstallPath)
		if err != nil {
			return bosherr.WrapErrorf(err, "Removing temporary compile directory %s", tmpInstallPath)
		}

		err = c.fs.MkdirAll(tmpInstallPath, os.FileMode(0755))
		if err != nil {
			return bosherr.WrapErrorf(err, "Creating temporary compile directory %s", tmpInstallPath)
		}
	}

	tmpInstallPathWithoutSymlinks, err := c.fs.ReadAndFollowLink(tmpInstallPath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Following Compile Path Symlink")
	}

	err = c.compressor.DecompressFileToDir(archivePath, tmpInstallPathWithoutSymlinks, boshcmd.CompressorOptions{})
	if err != nil {
		return bosherr.WrapErrorf(err, "Decompressing files from %s to %s", archivePath, tmpInstallPath)
	}

	return c.moveTmpDir(tmpInstallPath, finalDir)
}
