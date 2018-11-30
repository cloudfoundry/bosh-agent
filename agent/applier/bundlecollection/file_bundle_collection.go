package bundlecollection

import (
	"path"
	"path/filepath"
	"strings"

	"os"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-agent/agent/tarpath"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const fileBundleCollectionLogTag = "FileBundleCollection"

type fileBundleDefinition struct {
	name    string
	version string
}

func newFileBundleDefinition(installPath string) fileBundleDefinition {
	cleanInstallPath := cleanPath(installPath) // no trailing slash

	// If the path is empty, Base returns ".".
	// If the path consists entirely of separators, Base returns a single separator.

	name := path.Base(path.Dir(cleanInstallPath))
	if name == "." || name == string("/") {
		name = ""
	}

	version := path.Base(cleanInstallPath)
	if version == "." || version == string("/") {
		version = ""
	}

	return fileBundleDefinition{name: name, version: version}
}

func (bd fileBundleDefinition) BundleName() string    { return bd.name }
func (bd fileBundleDefinition) BundleVersion() string { return bd.version }

type FileBundleCollection struct {
	name         string
	installPath  string
	enablePath   string
	fileMode     os.FileMode
	fs           boshsys.FileSystem
	timeProvider clock.Clock
	compressor   fileutil.Compressor
	logger       boshlog.Logger
}

func NewFileBundleCollection(
	installPath, enablePath, name string,
	fileMode os.FileMode,
	fs boshsys.FileSystem,
	timeProvider clock.Clock,
	compressor fileutil.Compressor,
	logger boshlog.Logger,
) FileBundleCollection {
	return FileBundleCollection{
		name:         cleanPath(name),
		installPath:  cleanPath(installPath),
		enablePath:   cleanPath(enablePath),
		fileMode:     fileMode,
		fs:           fs,
		timeProvider: timeProvider,
		compressor:   compressor,
		logger:       logger,
	}
}

func (bc FileBundleCollection) Get(definition BundleDefinition) (Bundle, error) {
	if len(definition.BundleName()) == 0 {
		return nil, bosherr.Error("Missing bundle name")
	}

	if len(definition.BundleVersion()) == 0 {
		return nil, bosherr.Error("Missing bundle version")
	}

	bundleVersionDigest, err := boshcrypto.DigestAlgorithmSHA1.CreateDigest(strings.NewReader(definition.BundleVersion()))
	if err != nil {
		return FileBundle{}, err
	}

	installPath := path.Join(bc.installPath, bc.name, definition.BundleName(), bundleVersionDigest.String())
	enablePath := path.Join(bc.enablePath, bc.name, definition.BundleName())

	return NewFileBundle(installPath, enablePath, bc.fileMode, bc.fs, bc.timeProvider, bc.compressor, tarpath.NewPrefixDetector(), bc.logger), nil
}

func (bc FileBundleCollection) getDigested(definition BundleDefinition) (Bundle, error) {
	if len(definition.BundleName()) == 0 {
		return nil, bosherr.Error("Missing bundle name")
	}

	if len(definition.BundleVersion()) == 0 {
		return nil, bosherr.Error("Missing bundle version")
	}

	installPath := path.Join(bc.installPath, bc.name, definition.BundleName(), definition.BundleVersion())
	enablePath := path.Join(bc.enablePath, bc.name, definition.BundleName())
	return NewFileBundle(installPath, enablePath, bc.fileMode, bc.fs, bc.timeProvider, bc.compressor, tarpath.NewPrefixDetector(), bc.logger), nil
}

func (bc FileBundleCollection) List() ([]Bundle, error) {
	var bundles []Bundle

	bundleInstallPaths, err := bc.fs.Glob(path.Join(bc.installPath, bc.name, "*", "*"))
	if err != nil {
		return bundles, bosherr.WrapError(err, "Globbing bundles")
	}

	for _, path := range bundleInstallPaths {
		bundle, err := bc.getDigested(newFileBundleDefinition(path))
		if err != nil {
			return bundles, bosherr.WrapError(err, "Getting bundle")
		}

		bundles = append(bundles, bundle)
	}

	bc.logger.Debug(fileBundleCollectionLogTag, "Collection contains bundles %v", bundles)

	return bundles, nil
}

func cleanPath(name string) string {
	return path.Clean(filepath.ToSlash(name))
}
