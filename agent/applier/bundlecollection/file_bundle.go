package bundlecollection

import (
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/clock"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	"github.com/cloudfoundry/bosh-agent/v2/agent/tarpath"
)

const (
	fileBundleLogTag = "FileBundle"
)

type FileBundle struct {
	installPath  string
	enablePath   string
	fileMode     os.FileMode
	fs           boshsys.FileSystem
	timeProvider clock.Clock
	compressor   fileutil.Compressor
	detector     tarpath.Detector
	logger       boshlog.Logger
}

func NewFileBundle(
	installPath, enablePath string,
	fileMode os.FileMode,
	fs boshsys.FileSystem,
	timeProvider clock.Clock,
	compressor fileutil.Compressor,
	detector tarpath.Detector,
	logger boshlog.Logger,
) FileBundle {
	return FileBundle{
		installPath:  installPath,
		enablePath:   enablePath,
		fileMode:     fileMode,
		fs:           fs,
		timeProvider: timeProvider,
		compressor:   compressor,
		detector:     detector,
		logger:       logger,
	}
}

func (b FileBundle) InstallWithoutContents() (string, error) {
	b.logger.Debug(fileBundleLogTag, "Installing without contents %v", b)

	if err := b.fs.MkdirAll(b.installPath, b.fileMode); err != nil {
		return "", bosherr.WrapError(err, "Creating parent installation directory")
	}
	if err := b.fs.Chown(path.Dir(b.installPath), "root:vcap"); err != nil {
		_ = b.Uninstall() //nolint:errcheck
		return "", bosherr.WrapError(err, "Setting ownership on parent installation directory")
	}
	if err := b.fs.Chown(b.installPath, "root:vcap"); err != nil {
		_ = b.Uninstall() //nolint:errcheck
		return "", bosherr.WrapError(err, "Setting ownership on installation directory")
	}

	return b.installPath, nil
}

func (b FileBundle) Install(sourcePath, pathInBundle string) (string, error) {
	b.logger.Debug(fileBundleLogTag, "Installing %v", b)

	if _, err := b.InstallWithoutContents(); err != nil {
		return "", err
	}

	stripComponents := 0
	if pathInBundle != "" {
		// Job bundles contain more than one job. We receive the individual job's
		// path as pathInBundle but we don't want to have that be duplicated in the
		// installPath so we have tar strip that component out.
		stripComponents = 1

		// The structure of the tarball is different depending on whether or not it
		// was delivered over NATS or via the blobstore due to different archiving
		// code paths in the director. The NATS blobs do not contain a leading ./
		// path. We need to detect this case and extract the correct style of path.
		var err error
		hasSlash, err := b.detector.Detect(sourcePath, pathInBundle)
		if err != nil {
			_ = b.Uninstall() //nolint:errcheck
			return "", bosherr.WrapError(err, "Detecting prefix of package files")
		}

		if hasSlash {
			pathInBundle = "./" + pathInBundle
			stripComponents = 2
		}
	}

	installPathWithoutSymlinks, err := b.fs.ReadAndFollowLink(b.installPath)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Following Install Path Symlink")
	}

	err = b.compressor.DecompressFileToDir(
		sourcePath,
		installPathWithoutSymlinks,
		fileutil.CompressorOptions{PathInArchive: pathInBundle, StripComponents: stripComponents},
	)
	if err != nil {
		_ = b.Uninstall() //nolint:errcheck
		return "", bosherr.WrapError(err, "Decompressing package files")
	}

	b.logger.Debug(fileBundleLogTag, "Installing %v", b)
	return b.installPath, nil
}

func (b FileBundle) GetInstallPath() (string, error) {
	installPath := b.installPath
	if !b.fs.FileExists(installPath) {
		return "", bosherr.Error("install dir does not exist")
	}

	return installPath, nil
}

func (b FileBundle) IsInstalled() (bool, error) {
	return b.fs.FileExists(b.installPath), nil
}

func (b FileBundle) Enable() (string, error) {
	b.logger.Debug(fileBundleLogTag, "Enabling %v", b)

	if !b.fs.FileExists(b.installPath) {
		return "", bosherr.Error("bundle must be installed")
	}

	err := b.fs.MkdirAll(filepath.Dir(b.enablePath), b.fileMode)
	if err != nil {
		return "", bosherr.WrapError(err, "failed to create enable dir")
	}

	err = b.fs.Chown(filepath.Dir(b.enablePath), "root:vcap")
	if err != nil {
		return "", bosherr.WrapError(err, "Setting ownership on source directory")
	}

	err = b.fs.Symlink(b.installPath, b.enablePath)
	if err != nil {
		return "", bosherr.WrapError(err, "failed to enable")
	}

	return b.enablePath, nil
}

func (b FileBundle) Disable() error {
	b.logger.Debug(fileBundleLogTag, "Disabling %v", b)

	target, err := b.fs.ReadAndFollowLink(b.enablePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return bosherr.WrapError(err, "Reading symlink")
	}

	targetAbsPath, err := filepath.Abs(target)
	if err != nil {
		return bosherr.WrapError(err, "Determining absolute path")
	}

	installPath, err := b.fs.ReadAndFollowLink(b.installPath)
	if err != nil {
		return bosherr.WrapError(err, "Reading symlink")
	}

	installAbsPath, err := filepath.Abs(installPath)
	if err != nil {
		return bosherr.WrapError(err, "Determining absolute path")
	}

	if targetAbsPath == installAbsPath {
		return b.fs.RemoveAll(b.enablePath)
	}

	return nil
}
