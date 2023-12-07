package releasetarball_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry/bosh-cli/release/manifest"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	"gopkg.in/yaml.v2"

	"github.com/cloudfoundry/bosh-agent/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/agent/compiler"
	"github.com/cloudfoundry/bosh-agent/releasetarball"
	"github.com/cloudfoundry/bosh-agent/releasetarball/internal/fakes"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
)

func init() {
	log.Default().SetOutput(io.Discard)
}

func TestCompiler(t *testing.T) {
	d := directories.NewProvider(t.TempDir())

	_, err := releasetarball.NewCompiler(d)
	if err != nil {
		log.Fatal(err)
	}
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -fake-name Compiler -o internal/fakes/compiler.go github.com/cloudfoundry/bosh-agent/agent/compiler.Compiler

func TestCompile(t *testing.T) {
	t.Run("a release with multiple source packages", func(t *testing.T) {
		d := temporaryDirectoriesProvider(t)
		releasesOutputDir := t.TempDir()

		pkgCompiler := new(fakes.Compiler)
		pkgCompiler.CompileCalls(fakeCompilation(d))

		const stemcellSlug = "banana-slug/1.23"
		sourceBOSHReleaseFilepath := filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
		resultPath, err := releasetarball.Compile(pkgCompiler, sourceBOSHReleaseFilepath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
		if err != nil {
			t.Error(err)
		}
		if exp := filepath.Join(releasesOutputDir, "log-cache-3.0.9-banana-slug-1.23.tgz"); exp != resultPath {
			t.Errorf("unexpected resultPath: got %q expected %q", resultPath, exp)
		}

		const (
			expectedUname = "root"
			expectedUID   = 0
		)

		infos := listFileNamesInTarball(t, resultPath)
		const expectedFiles = 15
		if got, exp := len(infos), expectedFiles; got != exp {
			t.Fatalf("got %d files expected %d", got, exp)
		}
		for i, tt := range [expectedFiles]struct {
			Name string
			Mode int64
		}{
			{Name: "release.MF", Mode: 0o0644},
			{Name: "jobs/", Mode: 0o0755},
			{Name: "jobs/log-cache-gateway.tgz", Mode: 0o0644},
			{Name: "jobs/log-cache.tgz", Mode: 0o0644},
			{Name: "jobs/log-cache-syslog-server.tgz", Mode: 0o0644},
			{Name: "jobs/log-cache-cf-auth-proxy.tgz", Mode: 0o0644},
			{Name: "compiled_packages/", Mode: 0o0755},
			{Name: "compiled_packages/golang-1.20-linux.tgz", Mode: 0o0644},
			{Name: "compiled_packages/log-cache.tgz", Mode: 0o0644},
			{Name: "compiled_packages/log-cache-cf-auth-proxy.tgz", Mode: 0o0644},
			{Name: "compiled_packages/log-cache-gateway.tgz", Mode: 0o0644},
			{Name: "compiled_packages/log-cache-syslog-server.tgz", Mode: 0o0644},
			{Name: "license.tgz", Mode: 0o0644},
			{Name: "LICENSE", Mode: 0o0644},
			{Name: "NOTICE", Mode: 0o0644},
		} {
			t.Run(strings.TrimPrefix(tt.Name, "./"), func(t *testing.T) {
				h := infos[i]
				if got, exp := h.Name, "./"+tt.Name; got != exp {
					t.Errorf("expected file header name at index %d to be %q but got %q", i, got, exp)
				}
				requireOctal(t, h.Mode, tt.Mode)
				requireTime(t, h.ChangeTime, time.Time{})
				requireUnameAndUID(t, h, expectedUname, expectedUID)
			})
		}

		sourceManifest, err := releasetarball.Manifest(sourceBOSHReleaseFilepath)
		if err != nil {
			t.Fatal(err)
		}
		compiledManifest, err := releasetarball.Manifest(resultPath)
		if err != nil {
			t.Fatal(err)
		}
		if got, exp := compiledManifest.CommitHash, sourceManifest.CommitHash; got != exp {
			t.Errorf("wrong commit hash got %q expected %q", got, exp)
		}
		if got, exp := compiledManifest.Name, sourceManifest.Name; got != exp {
			t.Errorf("wrong release name got %q expected %q", got, exp)
		}
		if got, exp := compiledManifest.Version, sourceManifest.Version; got != exp {
			t.Errorf("wrong release version got %q expected %q", got, exp)
		}
		requireEqualJobs(t, compiledManifest.Jobs, sourceManifest.Jobs)
		if n := len(compiledManifest.Packages); n != 0 {
			t.Errorf("manifest has %d (source aka non-compiled) packages", n)
		}
		requirePackageCompilation(t, compiledManifest.CompiledPkgs, sourceManifest.Packages, stemcellSlug)
	})
	t.Run("a tarball with compiled packages", func(t *testing.T) {
		d := temporaryDirectoriesProvider(t)
		releasesOutputDir := t.TempDir()
		sourceTarballPath := filepath.Join("testdata", "log-cache-3.0.9-banana-slug-1.23.tgz")

		pkgCompiler := new(fakes.Compiler)
		pkgCompiler.CompileCalls(fakeCompilation(d))

		const stemcellSlug = "banana-slug/1.23"
		_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
		if got := pkgCompiler.CompileCallCount(); got != 0 {
			t.Errorf("compiler called %d times expected %d", got, 0)
		}
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("a tarball without packages", func(t *testing.T) {
		d := temporaryDirectoriesProvider(t)
		releasesOutputDir := t.TempDir()
		sourceTarballPath := filepath.Join("testdata", "bosh-dns-aliases-release-0.0.4.tgz")

		pkgCompiler := new(fakes.Compiler)
		pkgCompiler.CompileCalls(fakeCompilation(d))

		const stemcellSlug = "banana-slug/1.23"
		_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
		if got := pkgCompiler.CompileCallCount(); got != 0 {
			t.Errorf("compiler called %d times expected %d", got, 0)
		}
		if err != nil {
			t.Error(err)
		}
	})
	t.Run("a tarball with recursive package dependency", func(t *testing.T) {
		releaseMF, _ := yaml.Marshal(manifest.Manifest{
			Packages: []manifest.PackageRef{
				{Name: "A", Dependencies: []string{"B"}},
				{Name: "B", Dependencies: []string{"C"}},
				{Name: "C", Dependencies: []string{"A"}},
			},
		})
		tgz, err := createTGZ("release.MF", releaseMF, 0o0644)
		if err != nil {
			log.Fatal(err)
		}

		d := temporaryDirectoriesProvider(t)
		releasesOutputDir := t.TempDir()
		releaseInputDir := t.TempDir()
		sourceTarballPath := filepath.Join(releaseInputDir, "banana.tgz")
		if err := os.WriteFile(sourceTarballPath, tgz, 0o0644); err != nil {
			t.Fatal(err)
		}

		pkgCompiler := new(fakes.Compiler)
		pkgCompiler.CompileCalls(fakeCompilation(d))

		const stemcellSlug = "banana-slug/1.23"
		_, err = releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
		if err == nil || !strings.Contains(err.Error(), "cycle detected") {
			t.Errorf("expected a recursive tarball error got: %s", err)
		}
	})
}

func temporaryDirectoriesProvider(t *testing.T) directories.Provider {
	t.Helper()
	dir := t.TempDir()
	dp := directories.NewProvider(dir)

	if err := os.MkdirAll(dp.BlobsDir(), 0766); err != nil {
		log.Fatal(err)
	}

	return dp
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
}

func createTGZ(fileName string, content []byte, mode int64) ([]byte, error) {
	fn := func(w io.Writer) error {
		gw := gzip.NewWriter(w)
		defer closeAndIgnoreError(gw)
		tw := tar.NewWriter(gw)
		defer closeAndIgnoreError(tw)
		if err := tw.WriteHeader(&tar.Header{
			Typeflag:   tar.TypeReg,
			Name:       fileName,
			Size:       int64(len(content)),
			ModTime:    time.Time{},
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Mode:       mode,
		}); err != nil {
			return err
		}
		_, err := tw.Write(content)
		return err
	}
	w := new(bytes.Buffer)
	err := fn(w)
	return w.Bytes(), err
}

func listFileNamesInTarball(t *testing.T, filePath string) []tar.Header {
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAndIgnoreError(f)
	gr, err := gzip.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	tr := tar.NewReader(gr)
	var infos []tar.Header
	for {
		h, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		cp := *h
		cp.PAXRecords = maps.Clone(h.PAXRecords)
		infos = append(infos, cp)
		_, _ = io.Copy(io.Discard, tr)
	}
	return infos
}

func fakeCompilation(d directories.Provider) func(c compiler.Package, packages []models.Package) (string, boshcrypto.Digest, error) {
	return func(c compiler.Package, packages []models.Package) (string, boshcrypto.Digest, error) {
		blobContent, err := createTGZ("packaging", fmt.Appendf(nil, `"echo Compiled %q`, c.Name), 0o0744)
		if err != nil {
			log.Fatal(err)
		}
		compiledBlobstoreID := fmt.Sprintf("%s-compiled-blob", c.Name)
		if err := os.WriteFile(filepath.Join(d.BlobsDir(), compiledBlobstoreID), blobContent, 0o0644); err != nil {
			log.Fatal(err)
		}
		digester := sha1.New()
		_, _ = digester.Write(blobContent)
		digest := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, hex.EncodeToString(digester.Sum(nil)))
		return compiledBlobstoreID, digest, nil
	}
}

func requireOctal(t *testing.T, got, exp int64) {
	t.Helper()
	if got != exp {
		t.Errorf("wrong mode expected 0o%04o got 0o%04o", exp, got)
	}
}

func requireTime(t *testing.T, got, exp time.Time) {
	t.Helper()
	if !got.Equal(exp) {
		t.Errorf("wrong time expected %q got %q", exp, got)
	}
}

func requireEqualJobs(t *testing.T, gotJobs, expJobs []manifest.JobRef) {
	t.Helper()

	isEqual := slices.EqualFunc(gotJobs, expJobs, func(got manifest.JobRef, exp manifest.JobRef) bool {
		if reflect.DeepEqual(got, exp) {
			return true
		}
		t.Logf("jobs are not equal:\n\tgot: %#v\n\texp: %#v", got, exp)
		return false
	})
	if !isEqual {
		t.Fail()
	}
}

func requireUnameAndUID(t *testing.T, h tar.Header, expUname string, expUID int) {
	t.Helper()
	if got := h.Uname; got != expUname {
		t.Errorf("wrong uname expected %q got %q", expUname, got)
	}
	if got := h.Uid; got != expUID {
		t.Errorf("wrong uid expected %d got %d", expUID, got)
	}
}

func requirePackageCompilation(t *testing.T, compiled []manifest.CompiledPackageRef, source []manifest.PackageRef, stemcellSlug string) {
	t.Helper()
	if got, exp := len(compiled), len(source); got != exp {
		t.Errorf("got %d compiled packages expected %d", got, exp)
	}
	for i := range compiled {
		if got, exp := compiled[i].Name, source[i].Name; got != exp {
			t.Errorf("compiled package at index %d is has name %q but is expected to have %q", i, got, exp)
		}
		if got, exp := compiled[i].Version, source[i].Version; got != exp {
			t.Errorf("compiled package at index %d is has version %q but is expected to have %q", i, got, exp)
		}
		if got, exp := compiled[i].OSVersionSlug, stemcellSlug; got != exp {
			t.Errorf("compiled package at index %d is has stemcell %q but is expected to have %q", i, got, exp)
		}
		if got := compiled[i].SHA1; got == "" {
			t.Errorf("compiled package at index %d should have sha1 set %q", i, got)
		}
		if got, exp := compiled[i].Fingerprint, compiled[i].SHA1; got != exp {
			// I believe this is incorrect. I am "just" setting the fingerprint to the sha1 sum. This is wrong. See: https://bosh.io/docs/managing-releases/#jobs-and-packages
			t.Errorf("compiled package at index %d should have fingerprint %q to be identical to the sha1 sum %q", i, got, exp)
		}
		if got := compiled[i].Dependencies; len(got) != 0 {
			t.Errorf("compiled package at index %d should have no dependencies got %#v", i, got)
		}
	}
}
