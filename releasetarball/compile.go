// Package releasetarball encapsulates procedures to configure and run package
// compilation on via the BOSH agent executable. It should be invoked by running
//
//   bosh-agent compile [flags] [release.tgz...]
//
// To iterate on it in context of the executable see the script "bin/docker-run-bosh-agent".

package releasetarball

import (
	"archive/tar"
	"bufio"
	"bytes"
	"cmp"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"code.cloudfoundry.org/clock"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"

	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshmodels "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	boshap "github.com/cloudfoundry/bosh-agent/v2/agent/applier/packages"
	boshagentblobstore "github.com/cloudfoundry/bosh-agent/v2/agent/blobstore"
	boshrunner "github.com/cloudfoundry/bosh-agent/v2/agent/cmdrunner"
	boshcomp "github.com/cloudfoundry/bosh-agent/v2/agent/compiler"
	"github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider"
	"github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider/blobstore_delegator"
	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

const (
	releaseManifestFilename = "release.MF"
	defaultMode             = 0o0644
)

// NewCompiler can be used for multiple compilations and should be passed to Compile
// It expects to be used in a stemcell image and has not been tested on non-warden stemcells.
func NewCompiler(dirProvider directories.Provider) (boshcomp.Compiler, error) {
	logger := boshlog.New(boshlog.LevelWarn, log.Default())
	cmdRunner := boshsys.NewExecCmdRunner(logger)
	filesystem := boshsys.NewOsFileSystem(logger)
	compressor := boshcmd.NewTarballCompressor(cmdRunner, filesystem)
	blobstoreProvider := boshblob.NewProvider(filesystem, cmdRunner, dirProvider.EtcDir(), logger)
	db, err := blobstoreProvider.Get("local", map[string]any{"blobstore_path": dirProvider.BlobsDir()})
	if err != nil {
		return nil, err
	}
	bd := blobstore_delegator.NewBlobstoreDelegator(httpblobprovider.NewHTTPBlobImpl(filesystem, http.DefaultClient), boshagentblobstore.NewCascadingBlobstore(db, nil, logger), logger)
	ts := clock.NewClock()
	packageApplierProvider := boshap.NewCompiledPackageApplierProvider(dirProvider.DataDir(), dirProvider.BaseDir(), dirProvider.JobsDir(), "packages", bd, compressor, filesystem, ts, logger)
	const truncateLen = 10 * 1024 // 10kb
	runner := boshrunner.NewFileLoggingCmdRunner(filesystem, cmdRunner, dirProvider.LogsDir(), truncateLen)
	compiler := boshcomp.NewConcreteCompiler(compressor, bd, filesystem, runner, dirProvider, packageApplierProvider.Root(), packageApplierProvider.RootBundleCollection(), ts)
	return compiler, nil
}

// Compile expects the compiler returned by NewCompiler and may not work with compilers constructed differently.
func Compile(compiler boshcomp.Compiler, boshReleaseTarballPath, blobsDirectory, outputDirectory, stemcellSlug string) (string, error) {
	log.Printf("Reading BOSH Release Manifest from tarball %s", boshReleaseTarballPath)

	m, err := Manifest(boshReleaseTarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse release manifest: %w", err)
	}
	log.Printf("Release %s/%s has %d packages", m.Name, m.Version, len(m.Packages))

	log.Printf("Extracting packages")
	blobstoreIDs, err := extractPackages(m, blobsDirectory, boshReleaseTarballPath)
	if err != nil {
		return "", fmt.Errorf("failed to extract packages from tarball: %w", err)
	}

	log.Printf("Starting packages compilation")
	start := time.Now()
	packages := slices.Clone(m.Packages)
	if err := topologicalSort(packages, func(p manifest.PackageRef) string { return p.Name }, func(p manifest.PackageRef) []string { return slices.Clone(p.Dependencies) }); err != nil {
		return "", err
	}
	var compiledPackages []boshmodels.Package
	for _, p := range packages {
		compiledPackages, err = compilePackage(compiledPackages, p, blobstoreIDs, compiler)
		if err != nil {
			return "", fmt.Errorf("failed to compile package %s/%s: %w", p.Name, p.Version, err)
		}
	}
	log.Printf("Finished packages compilation after %s", time.Since(start))

	log.Printf("Archiving compiled BOSH Release %s/%s with stemcell %s", m.Name, m.Version, stemcellSlug)
	return writeCompiledRelease(m, outputDirectory, stemcellSlug, blobsDirectory, boshReleaseTarballPath, m.Packages, compiledPackages)
}

func compilePackage(compiledPackages []boshmodels.Package, p manifest.PackageRef, blobstoreIDs map[string]string, compiler boshcomp.Compiler) ([]boshmodels.Package, error) {
	log.Printf("Compiling package %s/%s", p.Name, p.Version)
	digest, err := boshcrypto.ParseMultipleDigest(p.SHA1)
	if err != nil {
		return nil, err
	}
	pkg := boshcomp.Package{
		BlobstoreID: path.Base(blobstoreIDs[p.SHA1]),
		Name:        p.Name,
		Sha1:        digest,
		Version:     p.Version,
	}
	modelsDeps := make([]boshmodels.Package, 0, len(p.Dependencies))
	for _, dep := range p.Dependencies {
		index := slices.IndexFunc(compiledPackages, func(b boshmodels.Package) bool {
			return b.Name == dep
		})
		modelsDeps = append(modelsDeps, compiledPackages[index])
	}
	compiledBlobID, compiledDigest, err := compiler.Compile(pkg, modelsDeps)
	if err != nil {
		return nil, err
	}
	log.Printf("Finished compiling release %s/%s BlobstoreID=%s", p.Name, p.Version, compiledBlobID)
	return append(compiledPackages, boshmodels.Package{
		Name:    pkg.Name,
		Version: pkg.Version,
		Source: boshmodels.Source{
			Sha1:        compiledDigest,
			BlobstoreID: compiledBlobID,
		},
	}), nil
}

func extractPackages(m manifest.Manifest, blobsDirectory, releaseTarballPath string) (map[string]string, error) {
	sha1ToFilepath := make(map[string]string, len(m.Packages))
	if err := walkTarballFiles(releaseTarballPath, func(name string, h *tar.Header, src io.Reader) (bool, error) {
		if match, err := path.Match("packages/*.tgz", name); err != nil || !match {
			return true, err
		}
		pkgIndex := slices.IndexFunc(m.Packages, func(p manifest.PackageRef) bool {
			return p.Name+".tgz" == path.Base(h.Name)
		})
		if pkgIndex < 0 {
			return true, fmt.Errorf("package not found in release manifest")
		}
		p := m.Packages[pkgIndex]
		dstFilepath := filepath.Join(blobsDirectory, p.Name+".tgz")
		digest, err := boshcrypto.ParseMultipleDigest(p.SHA1)
		if err != nil {
			return false, err
		}
		dst, err := os.OpenFile(dstFilepath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultMode)
		if err != nil {
			return false, err
		}
		defer closeAndIgnoreErr(dst)
		_, err = io.Copy(dst, src)
		log.Printf("Copying package %s to %s", p.Name, dstFilepath)
		if err != nil {
			return false, errors.Join(err, os.RemoveAll(dstFilepath))
		}
		_, err = dst.Seek(0, 0)
		if err != nil {
			return false, errors.Join(err, os.RemoveAll(dstFilepath))
		}
		if err := digest.Verify(dst); err != nil {
			return false, errors.Join(err, os.RemoveAll(dstFilepath))
		}
		sha1ToFilepath[p.SHA1] = dstFilepath
		return true, nil
	}); err != nil {
		return nil, err
	}
	return sha1ToFilepath, nil
}

func newCompiledPackageRef(pkg boshmodels.Package, src manifest.PackageRef, stemcellSlug string) manifest.CompiledPackageRef {
	compiled := manifest.CompiledPackageRef{
		Name:          pkg.Name,
		Version:       pkg.Version,
		Fingerprint:   src.Fingerprint,
		SHA1:          pkg.Source.Sha1.String(),
		OSVersionSlug: stemcellSlug,
		Dependencies:  slices.Clone(src.Dependencies),
	}
	return compiled
}

func writeCompiledRelease(m manifest.Manifest, outputDirectory, stemcellFilenameSuffix, blobsDirectory, initialTarball string, sourcePackages []manifest.PackageRef, compiledPackages []boshmodels.Package) (string, error) {
	m.CompiledPkgs = make([]manifest.CompiledPackageRef, 0, len(compiledPackages))
	for _, p := range compiledPackages {
		srcIndex := slices.IndexFunc(sourcePackages, func(ref manifest.PackageRef) bool {
			return ref.Name == p.Name && ref.Version == p.Version
		})
		m.CompiledPkgs = append(m.CompiledPkgs, newCompiledPackageRef(p, sourcePackages[srcIndex], stemcellFilenameSuffix))
	}
	empty := [0]manifest.PackageRef{}
	m.Packages = empty[:]

	releaseManifestBuffer, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("%s-%s-%s.tgz", m.Name, m.Version, strings.Replace(stemcellFilenameSuffix, "/", "-", 1))
	filePath := filepath.Join(outputDirectory, fileName)
	outputFile, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer closeAndIgnoreErr(outputFile)

	var tw *tar.Writer
	if m.NoCompression {
		tw = tar.NewWriter(outputFile)
	} else {
		gw := gzip.NewWriter(outputFile)
		defer closeAndIgnoreErr(gw)
		tw = tar.NewWriter(gw)
	}
	defer closeAndIgnoreErr(tw)

	err = walkTarballFiles(initialTarball, writeCompiledTarballFiles(tw, compiledPackages, releaseManifestBuffer, blobsDirectory))
	if err != nil {
		return "", errors.Join(err, os.RemoveAll(filePath))
	}
	return filePath, nil
}

func writeCompiledTarballFiles(tw *tar.Writer, compiledPackages []boshmodels.Package, releaseManifestBuffer []byte, blobsDirectory string) tarballWalkFunc {
	return func(fullPath string, h *tar.Header, r io.Reader) (bool, error) {
		switch {
		case h.FileInfo().Name() == releaseManifestFilename:
			h.Size = int64(len(releaseManifestBuffer))
			if err := writeToTar(tw, h, fullPath, bytes.NewReader(releaseManifestBuffer)); err != nil {
				return false, fmt.Errorf("failed to write release.MF header: %w", err)
			}
			return true, nil
		case h.FileInfo().IsDir() && path.Base(path.Dir(fullPath)) == "packages":
			fullPath = "compiled_packages/"
			h.Name = fullPath
			if err := writeToTar(tw, h, fullPath, r); err != nil {
				return false, fmt.Errorf("failed to write compiled_packages directory: %w", err)
			}
			for _, p := range compiledPackages {
				pkgOSFilePath := filepath.Join(blobsDirectory, p.Source.BlobstoreID)
				pkgTGZFilePath := path.Join("compiled_packages", fmt.Sprintf("%s.tgz", p.Name))
				if err := insertFile(tw, h, pkgTGZFilePath, pkgOSFilePath); err != nil {
					return false, fmt.Errorf("failed to write compiled package %s: %w", p.Name, err)
				}
			}
			return true, nil
		case !h.FileInfo().IsDir() && path.Base(path.Dir(fullPath)) == "packages":
			return true, nil // skip source packages
		default:
			if !h.FileInfo().IsDir() {
				h.Mode = defaultMode
			}
			return true, writeToTar(tw, h, fullPath, r)
		}
	}
}

func insertFile(tw *tar.Writer, dir *tar.Header, tgzFilePath, osFilePath string) error {
	f, err := os.Open(osFilePath)
	if err != nil {
		return err
	}
	defer closeAndIgnoreErr(f)
	info, err := f.Stat()
	if err != nil {
		return err
	}
	h, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	h.Mode = defaultMode
	clearTimestamps(h)
	copyUserHeaderFields(h, dir)
	return writeToTar(tw, h, tgzFilePath, f)
}

func Manifest(releaseFilePath string) (manifest.Manifest, error) {
	var (
		m     manifest.Manifest
		found = false
	)
	err := walkTarballFiles(releaseFilePath, readReleaseManifest(&found, &m))
	if !found {
		return m, fmt.Errorf("failed to find %s in tarball", releaseManifestFilename)
	}
	return m, err
}

func readReleaseManifest(found *bool, m *manifest.Manifest) func(string, *tar.Header, io.Reader) (bool, error) {
	return func(name string, _ *tar.Header, r io.Reader) (bool, error) {
		if path.Base(name) != "release.MF" {
			return true, nil
		}
		*found = true
		buf, err := io.ReadAll(r)
		if err != nil {
			return false, err
		}
		return false, yaml.Unmarshal(buf, m)
	}
}

type tarballWalkFunc func(fullName string, h *tar.Header, r io.Reader) (bool, error)

func walkTarballFiles(releaseFilePath string, file tarballWalkFunc) error {
	f, err := os.Open(releaseFilePath)
	if err != nil {
		return nil
	}
	defer closeAndIgnoreErr(f)

	// Try to read as gzipped first, fall back to uncompressed if it fails
	var r *tar.Reader
	gr, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		// Not gzipped, read as uncompressed tar
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		r = tar.NewReader(f)
	} else {
		// Successfully opened as gzipped
		r = tar.NewReader(gr)
	}

	found := 0
	for {
		h, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		found++
		if keepGoing, err := file(h.Name, cloneHeader(h), r); err != nil {
			return err
		} else if !keepGoing {
			return nil
		}
	}
}

func cloneHeader(in *tar.Header) *tar.Header {
	result := new(tar.Header)
	*result = *in // shallow copy
	in.PAXRecords = maps.Clone(in.PAXRecords)
	return result
}

func copyUserHeaderFields(out, in *tar.Header) {
	out.Uid = in.Uid
	out.Gid = in.Gid
	out.Uname = in.Uname
	out.Gname = in.Gname
}

func clearTimestamps(h *tar.Header) {
	h.AccessTime = time.Time{}
	h.ChangeTime = time.Time{}
	h.ModTime = time.Time{}
}

func writeToTar(tw *tar.Writer, h *tar.Header, fullPath string, r io.Reader) error {
	clearTimestamps(h)
	h.Name = "./" + fullPath
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := io.Copy(tw, r)
	if err != nil {
		return err
	}
	return nil
}

func closeAndIgnoreErr(c io.Closer) {
	_ = c.Close() //nolint:errcheck
}

func topologicalSort[T any, ID cmp.Ordered, IDFunc func(T) ID, EdgeFunc func(T) []ID](elements []T, elementID IDFunc, elementEdges EdgeFunc) error {
	var (
		visited   = make([]bool, 2*len(elements))
		temporal  = visited[:len(elements)]
		permanent = visited[len(elements):]

		ids    = make(map[ID]int, len(elements))
		sorted = make([]T, 0, len(elements))
	)
	var visit func(ID) error
	visit = func(id ID) error {
		if permanent[ids[id]] {
			return nil
		}
		if temporal[ids[id]] {
			return fmt.Errorf("cycle detected")
		}
		temporal[ids[id]] = true
		e := elements[ids[id]]
		for _, dep := range elementEdges(e) {
			if err := visit(dep); err != nil {
				return err
			}
		}
		sorted = append(sorted, e)
		permanent[ids[id]] = true
		return nil
	}
	slices.SortFunc(elements, func(a, b T) int {
		return cmp.Compare(elementID(a), elementID(b))
	})
	for i, e := range elements {
		ids[elementID(e)] = i
	}
	for _, e := range elements {
		if err := visit(elementID(e)); err != nil {
			return err
		}
	}
	copy(elements, sorted)
	return nil
}
