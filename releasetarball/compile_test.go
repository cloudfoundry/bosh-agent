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
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	"gopkg.in/yaml.v3"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/v2/agent/compiler"
	"github.com/cloudfoundry/bosh-agent/v2/releasetarball"
	"github.com/cloudfoundry/bosh-agent/v2/releasetarball/internal/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

func TestSuite(t *testing.T) {
	log.Default().SetOutput(io.Discard)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Release Compilation Suite")
}

var _ = Describe("NewCompiler", func() {
	When("initialized", func() {
		var (
			temporaryDirectory string
			setupErr           error
		)

		BeforeEach(func() {
			temporaryDirectory, setupErr = os.MkdirTemp("", "")
		})
		AfterEach(func() {
			setupErr = errors.Join(setupErr, os.RemoveAll(temporaryDirectory))
		})

		It("returns a result and no error", func() {
			Expect(setupErr).NotTo(HaveOccurred())

			d := directories.NewProvider(temporaryDirectory)
			Expect(os.MkdirAll(d.BlobsDir(), 0o766)).To(Succeed())

			result, err := releasetarball.NewCompiler(d)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
		})
	})
})

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -fake-name Compiler -o internal/fakes/compiler.go github.com/cloudfoundry/bosh-agent/v2/agent/compiler.Compiler

var _ = Describe("Compile", func() {
	const stemcellSlug = "banana-slug/1.23"

	var (
		releasesOutputDir string

		pkgCompiler *fakes.Compiler

		sourceTarballPath string

		d directories.Provider
	)

	BeforeEach(func() {
		d = directories.NewProvider(GinkgoT().TempDir())
		Expect(os.MkdirAll(d.BlobsDir(), 0o766)).To(Succeed())
		releasesOutputDir = GinkgoT().TempDir()
		pkgCompiler = new(fakes.Compiler)
		pkgCompiler.CompileCalls(fakeCompilation(d))
	})

	When("compiling a tarball with compiled packages", func() {
		BeforeEach(func() {
			sourceTarballPath = filepath.Join("testdata", "log-cache-3.0.9-banana-slug-1.23.tgz")
		})

		It("does not compile any of the packages", func() {
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pkgCompiler.CompileCallCount()).To(BeZero())
		})
	})

	It("ships every package without compile-time metadata when not opted in", func() {
		sourceTarballPath = filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
		source := mustReleaseManifest(sourceTarballPath)
		resultPath, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
		Expect(err).NotTo(HaveOccurred())
		compiled, err := releasetarball.Manifest(resultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(compiled.Version).To(Equal(source.Version))
		Expect(compiled.CompiledPkgs).To(HaveLen(len(source.Packages)))
		for _, p := range compiled.CompiledPkgs {
			Expect(p.OriginalFingerprint).To(BeEmpty(), p.Name)
		}
		Expect(compiled.CompileTimePackages).To(BeNil())
		raw, err := releaseManifestBytes(resultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("compile_time_packages"))
		Expect(string(raw)).NotTo(ContainSubstring("original_fingerprint"))
	})

	It("strips compile-time packages and rewrites the shipped identities when opted in", func() {
		sourceTarballPath = filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
		resultPath, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Base(resultPath)).To(ContainSubstring("3.0.9+trimmed"))
		Expect(pkgCompiler.CompileCallCount()).To(Equal(5))

		source, err := releasetarball.Manifest(sourceTarballPath)
		Expect(err).NotTo(HaveOccurred())
		compiled, err := releasetarball.Manifest(resultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(compiled.Version).To(Equal("3.0.9+trimmed"))
		Expect(compiled.Jobs).To(Equal(source.Jobs))
		Expect(compiled.CompiledPkgs).To(HaveLen(4))
		Expect(compiled.CompiledPkgs).To(ConsistOf(
			HaveField("Name", "log-cache"), HaveField("Name", "log-cache-cf-auth-proxy"),
			HaveField("Name", "log-cache-gateway"), HaveField("Name", "log-cache-syslog-server"),
		))
		Expect(compiled.CompileTimePackages).To(HaveLen(1))
		Expect(compiled.CompileTimePackages[0].Name).To(Equal(source.Packages[0].Name))
		Expect(compiled.CompileTimePackages[0].Version).To(Equal(source.Packages[0].Version))
		Expect(compiled.CompileTimePackages[0].Fingerprint).To(Equal(source.Packages[0].Fingerprint))
		Expect(compiled.CompileTimePackages[0].Dependencies).To(Equal(source.Packages[0].Dependencies))
		rawManifest, err := releaseManifestBytes(resultPath)
		Expect(err).NotTo(HaveOccurred())
		if capturePath := os.Getenv("CAPTURE_COMPILE_MANIFEST"); capturePath != "" {
			Expect(os.WriteFile(capturePath, rawManifest, 0o644)).To(Succeed())
		}
		compileTimeSection := strings.SplitN(string(rawManifest), "compile_time_packages:\n", 2)[1]
		if strings.Contains(compileTimeSection, "\nlicense:") {
			compileTimeSection = strings.SplitN(compileTimeSection, "\nlicense:", 2)[0]
		}
		Expect(compileTimeSection).NotTo(ContainSubstring("sha1:"))

		rewritten, err := releasetarball.RewriteIdentitiesForTest(source.Packages, map[string]bool{
			"log-cache": true, "log-cache-cf-auth-proxy": true, "log-cache-gateway": true, "log-cache-syslog-server": true,
		})
		Expect(err).NotTo(HaveOccurred())
		for _, p := range compiled.CompiledPkgs {
			identity := rewritten[p.Name]
			Expect(p.Version).To(Equal(identity.Fingerprint))
			Expect(p.Fingerprint).To(Equal(identity.Fingerprint))
			Expect(p.OriginalFingerprint).To(Equal(identity.OriginalFingerprint))
			Expect(p.Fingerprint).NotTo(Equal(p.OriginalFingerprint))
			Expect(p.Dependencies).To(Equal(identity.Dependencies))
		}

		entries := listFileNamesInTarball(GinkgoT(), resultPath)
		var compiledNames []string
		for _, entry := range entries {
			if filepath.Base(filepath.Dir(entry.Name)) == "compiled_packages" && entry.Name != "./compiled_packages/" {
				compiledNames = append(compiledNames, filepath.Base(entry.Name))
			}
		}
		Expect(compiledNames).To(ConsistOf("log-cache.tgz", "log-cache-cf-auth-proxy.tgz", "log-cache-gateway.tgz", "log-cache-syslog-server.tgz"))
	})

	It("writes sorted dropped packages with original dependency order and retains unaffected package fields", func() {
		fpA := strings.Repeat("a", 40)
		fpB := strings.Repeat("b", 40)
		sourcePackages := []manifest.PackageRef{
			{Name: "keep-a", Version: "version-a", Fingerprint: fpA, SHA1: "source-sha-a"},
			{Name: "drop-z", Version: "version-z", Fingerprint: "fp-z", Dependencies: []string{}},
			{Name: "keep-b", Version: "version-b", Fingerprint: fpB, SHA1: "source-sha-b", Dependencies: []string{"drop-z", "keep-a"}},
			{Name: "drop-a", Version: "version-drop-a", Fingerprint: "fp-drop-a", Dependencies: []string{"drop-z", "keep-b"}},
			{Name: "drop-m", Version: "version-drop-m", Fingerprint: "fp-drop-m", Dependencies: []string{"drop-a", "drop-z"}},
		}
		blobs := GinkgoT().TempDir()
		compiled := make([]models.Package, 0, 2)
		for _, name := range []string{"keep-a", "keep-b"} {
			blobID := name + ".tgz"
			Expect(os.WriteFile(filepath.Join(blobs, blobID), []byte("compiled blob"), 0o644)).To(Succeed())
			digest := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, strings.Repeat("c", 40))
			compiled = append(compiled, models.Package{Name: name, Version: "version-" + strings.TrimPrefix(name, "keep-"), Source: models.Source{Sha1: digest, BlobstoreID: blobID}})
		}
		m := manifest.Manifest{Name: "synthetic", Version: "1.0", Packages: sourcePackages}
		result, err := releasetarball.WriteCompiledReleaseForTest(m, GinkgoT().TempDir(), stemcellSlug, blobs, filepath.Join("testdata", "log-cache-release-3.0.9.tgz"), sourcePackages, compiled, map[string]bool{"keep-a": true, "keep-b": true})
		Expect(err).NotTo(HaveOccurred())
		written, err := releasetarball.Manifest(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(written.CompileTimePackages).To(Equal([]manifest.CompileTimePackageRef{
			{Name: "drop-a", Version: "version-drop-a", Fingerprint: "fp-drop-a", Dependencies: []string{"drop-z", "keep-b"}},
			{Name: "drop-m", Version: "version-drop-m", Fingerprint: "fp-drop-m", Dependencies: []string{"drop-a", "drop-z"}},
			{Name: "drop-z", Version: "version-z", Fingerprint: "fp-z", Dependencies: []string{}},
		}))
		unaffected := written.CompiledPkgs[0]
		Expect(unaffected.Name).To(Equal("keep-a"))
		Expect(unaffected.Version).To(Equal("version-a"))
		Expect(unaffected.Fingerprint).To(Equal(fpA))
		Expect(unaffected.SHA1).To(Equal(strings.Repeat("c", 40)))
		Expect(unaffected.OSVersionSlug).To(Equal(stemcellSlug))
		Expect(unaffected.Dependencies).To(BeEmpty())
		Expect(unaffected.OriginalFingerprint).To(BeEmpty())
	})

	It("uses a dot suffix for release versions that already contain build metadata", func() {
		release := mustReleaseManifest(filepath.Join("testdata", "log-cache-release-3.0.9.tgz"))
		release.Version = "1.0+dev.3"
		sourceTarballPath = rewriteReleaseManifest(GinkgoT(), filepath.Join("testdata", "log-cache-release-3.0.9.tgz"), release)
		resultPath, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Base(resultPath)).To(ContainSubstring("1.0+dev.3.trimmed"))
		compiled, err := releasetarball.Manifest(resultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(compiled.Version).To(Equal("1.0+dev.3.trimmed"))
	})

	It("omits compile-time metadata when every package is shipped", func() {
		sourceTarballPath = filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
		source := mustReleaseManifest(sourceTarballPath)
		source.CompileTimePackages = []manifest.CompileTimePackageRef{{Name: "stale", Version: "0", Fingerprint: "stale"}}
		sourceTarballPath = rewriteReleaseManifest(GinkgoT(), sourceTarballPath, source)
		allPackages := make([]string, 0, len(source.Packages))
		for _, p := range source.Packages {
			allPackages = append(allPackages, p.Name)
		}
		sourceTarballPath = rewriteJobPackages(GinkgoT(), sourceTarballPath, allPackages)
		resultPath, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).NotTo(HaveOccurred())
		compiled, err := releasetarball.Manifest(resultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(compiled.Version).To(Equal(source.Version))
		Expect(compiled.CompileTimePackages).To(BeNil())
		raw, err := releaseManifestBytes(resultPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("compile_time_packages"))
		Expect(string(raw)).NotTo(ContainSubstring("original_fingerprint"))
	})

	It("preserves zero-package release bytes when the option is enabled", func() {
		sourceTarballPath = writeShippedRelease(GinkgoT(), manifest.Manifest{Name: "empty", Version: "1.0"}, nil, false)
		withoutFlag, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
		Expect(err).NotTo(HaveOccurred())
		withoutFlagBytes, err := os.ReadFile(withoutFlag)
		Expect(err).NotTo(HaveOccurred())
		withFlag, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).NotTo(HaveOccurred())
		withFlagBytes, err := os.ReadFile(withFlag)
		Expect(err).NotTo(HaveOccurred())
		Expect(withFlagBytes).To(Equal(withoutFlagBytes))
	})

	It("preserves already-compiled releases when the option is enabled", func() {
		sourceTarballPath = filepath.Join("testdata", "log-cache-3.0.9-banana-slug-1.23.tgz")
		withoutFlag, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
		Expect(err).NotTo(HaveOccurred())
		withoutFlagBytes, err := os.ReadFile(withoutFlag)
		Expect(err).NotTo(HaveOccurred())
		withFlag, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).NotTo(HaveOccurred())
		withFlagBytes, err := os.ReadFile(withFlag)
		Expect(err).NotTo(HaveOccurred())
		Expect(withFlagBytes).To(Equal(withoutFlagBytes))
	})

	It("preserves zero-package releases with jobs when the option is enabled", func() {
		sourceTarballPath = writeShippedRelease(GinkgoT(), manifest.Manifest{
			Name: "no-packages", Version: "1.0.0", Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "name: job\npackages: []\n"}, false)
		withoutFlag, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
		Expect(err).NotTo(HaveOccurred())
		withoutFlagBytes, err := os.ReadFile(withoutFlag)
		Expect(err).NotTo(HaveOccurred())
		withFlag, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).NotTo(HaveOccurred())
		withFlagBytes, err := os.ReadFile(withFlag)
		Expect(err).NotTo(HaveOccurred())
		Expect(withFlagBytes).To(Equal(withoutFlagBytes))
	})

	It("fails before compilation or output when a job references an unknown package", func() {
		sourceTarballPath = writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "known", Version: "1", Fingerprint: "0123456789012345678901234567890123456789"}},
			Jobs:     []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "name: job\npackages: [unknown]\n"}, false)
		_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
		Expect(err).To(MatchError(ContainSubstring("unknown package")))
		Expect(pkgCompiler.CompileCallCount()).To(BeZero())
		entries, err := os.ReadDir(releasesOutputDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	It("fails before compilation or output for missing job archives, missing job.MF, and an empty shipped set", func() {
		fixture := filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
		tests := []struct {
			name  string
			input func() string
			want  string
		}{
			{
				name:  "missing archive",
				input: func() string { return removeJobArchives(GinkgoT(), fixture) },
				want:  "missing job archive",
			},
			{
				name:  "missing job.MF",
				input: func() string { return replaceJobArchives(GinkgoT(), fixture) },
				want:  "no job.MF",
			},
			{
				name:  "empty shipped set",
				input: func() string { return rewriteJobPackages(GinkgoT(), fixture, nil) },
				want:  "no shipped packages",
			},
		}
		for _, tt := range tests {
			By(tt.name)
			output := GinkgoT().TempDir()
			pkgCompiler = new(fakes.Compiler)
			pkgCompiler.CompileCalls(fakeCompilation(d))
			_, err := releasetarball.Compile(pkgCompiler, tt.input(), d.BlobsDir(), output, stemcellSlug, releasetarball.CompileOptions{ExcludeCompileTimePackages: true})
			Expect(err).To(MatchError(ContainSubstring(tt.want)))
			Expect(pkgCompiler.CompileCallCount()).To(BeZero())
			entries, err := os.ReadDir(output)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		}
	})

	When("compiling a tarball with a package dependency cycle", func() {
		BeforeEach(func() {
			releaseInputDir := GinkgoT().TempDir()
			sourceTarballPath = filepath.Join(releaseInputDir, "banana.tgz")

			releaseMF, _ := yaml.Marshal(manifest.Manifest{ //nolint:errcheck
				Packages: []manifest.PackageRef{
					{Name: "A", Dependencies: []string{"B"}},
					{Name: "B", Dependencies: []string{"C"}},
					{Name: "C", Dependencies: []string{"A"}},
				},
			})
			tgz, err := createTGZ(simpleFile("release.MF", releaseMF, 0o0644))
			Expect(err).NotTo(HaveOccurred())
			if err := os.WriteFile(sourceTarballPath, tgz, 0o0644); err != nil {
				panic(err)
			}
		})

		It("does not compile any of the packages", func() {
			const stemcellSlug = "banana-slug/1.23"
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
			Expect(err).To(MatchError(ContainSubstring("cycle detected")))
		})
	})

	When("the release manifest is invalid", func() {
		BeforeEach(func() {
			releaseInputDir := GinkgoT().TempDir()
			sourceTarballPath = filepath.Join(releaseInputDir, "banana.tgz")

			releaseMF := []byte(`{"name": ["wrong type for name field"]}`)
			tgz, err := createTGZ(simpleFile("release.MF", releaseMF, 0o0644))
			Expect(err).NotTo(HaveOccurred())
			if err := os.WriteFile(sourceTarballPath, tgz, 0o0644); err != nil {
				panic(err)
			}
		})

		It("does not compile any of the packages", func() {
			const stemcellSlug = "banana-slug/1.23"
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
			Expect(err).To(MatchError(ContainSubstring("failed to parse release manifest")))
		})
	})

	When("the compiler returns an error", func() {
		BeforeEach(func() {
			sourceTarballPath = filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
			pkgCompiler.CompileReturns("", nil, fmt.Errorf("banana"))
		})

		It("does not compile any of the packages", func() {
			const stemcellSlug = "banana-slug/1.23"
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
			Expect(err).To(MatchError(ContainSubstring("banana")))
		})
	})

	When("package tarball is not found in release manifest", func() {
		BeforeEach(func() {
			releaseInputDir := GinkgoT().TempDir()
			sourceTarballPath = filepath.Join(releaseInputDir, "banana.tgz")
			releaseMF, _ := yaml.Marshal(manifest.Manifest{}) //nolint:errcheck

			tgz, err := createTGZ(
				simpleFile("release.MF", releaseMF, 0o0644),
				simpleFile("packages/a.tgz", nil, 0o0644),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(sourceTarballPath, tgz, 0o0644)).To(Succeed())
		})

		It("returns a helpful error", func() {
			const stemcellSlug = "banana-slug/1.23"
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
			Expect(err).To(MatchError(ContainSubstring("package not found in release manifest")))
		})
	})

	When("compiling a release with multiple source packages", func() {
		var multiplePackageCompiler *fakes.Compiler
		BeforeEach(func() {
			sourceTarballPath = filepath.Join("testdata", "log-cache-release-3.0.9.tgz")

			multiplePackageCompiler = new(fakes.Compiler)
			for i, blob := range []string{
				"golang-1.20-linux-blob",
				"log-cache-blob",
				"log-cache-cf-auth-proxy-blob",
				"log-cache-gateway-blob",
				"log-cache-syslog-server-blob",
			} {
				p := filepath.Join(d.BlobsDir(), blob)
				Expect(os.WriteFile(p, []byte(blob), 0644)).To(Succeed())
				multiplePackageCompiler.CompileReturnsOnCall(i, blob, boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, blob+"-checksum"), nil)
			}
		})

		It("writes a compiled release tarball", func() {
			resultPath, err := releasetarball.Compile(multiplePackageCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug, releasetarball.CompileOptions{})
			Expect(err).To(Succeed())

			Expect(multiplePackageCompiler.CompileCallCount()).To(Equal(5))

			By("generating a useful filename", func() {
				Expect(resultPath).To(Equal(filepath.Join(releasesOutputDir, "log-cache-3.0.9-banana-slug-1.23.tgz")))
			})

			By("mutating the release manifest", func() {
				sourceManifest, err := releasetarball.Manifest(sourceTarballPath)
				Expect(err).NotTo(HaveOccurred())

				compiledManifest, err := releasetarball.Manifest(resultPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(compiledManifest.CommitHash).To(Equal(sourceManifest.CommitHash), "it must not change the commit sha")
				Expect(compiledManifest.Name).To(Equal(sourceManifest.Name), "it must not change the name")
				Expect(compiledManifest.Version).To(Equal(sourceManifest.Version), "it must not change the version")

				Expect(compiledManifest.Jobs).To(Equal(sourceManifest.Jobs), "it must not change the jobs")
				Expect(compiledManifest.Packages).To(HaveLen(0), "it must not leave any source packages")

				Expect(compiledManifest.CompiledPkgs).To(HaveLen(len(sourceManifest.Packages)), "it should convert all the source packages to compiled packages")

				assertPackageFields(compiledManifest, sourceManifest, 0, stemcellSlug, "golang-1.20-linux-blob-checksum")
				assertPackageFields(compiledManifest, sourceManifest, 1, stemcellSlug, "log-cache-blob-checksum")
				assertPackageFields(compiledManifest, sourceManifest, 2, stemcellSlug, "log-cache-cf-auth-proxy-blob-checksum")
				assertPackageFields(compiledManifest, sourceManifest, 3, stemcellSlug, "log-cache-gateway-blob-checksum")
				assertPackageFields(compiledManifest, sourceManifest, 4, stemcellSlug, "log-cache-syslog-server-blob-checksum")
			})

			infos := listFileNamesInTarball(GinkgoT(), resultPath)
			const expectedFiles = 15

			Expect(infos).To(HaveLen(expectedFiles))
			const (
				expectedUname = "root"
				expectedUID   = 0
			)
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
				h := infos[i]
				Expect(h.Name).To(Equal("./"+tt.Name), fmt.Sprintf("expected file header name at index %d", i))
				Expect(h.Mode).To(Equal(tt.Mode))
				Expect(h.ChangeTime.IsZero()).To(BeTrue())
				Expect(h.ChangeTime.IsZero()).To(BeTrue())
				Expect(h.Uname).To(Equal(expectedUname))
				Expect(h.Uid).To(Equal(expectedUID))
			}
		})
	})
})

func assertPackageFields(compiledManifest, sourceManifest manifest.Manifest, index int, stemcellSlug, expectedSHA string) {
	GinkgoHelper()

	Expect(compiledManifest.CompiledPkgs[index].Name).To(Equal(sourceManifest.Packages[index].Name))
	Expect(compiledManifest.CompiledPkgs[index].Version).To(Equal(sourceManifest.Packages[index].Version))
	Expect(compiledManifest.CompiledPkgs[index].Fingerprint).To(Equal(sourceManifest.Packages[index].Fingerprint))
	Expect(compiledManifest.CompiledPkgs[index].OSVersionSlug).To(Equal(stemcellSlug))
	Expect(compiledManifest.CompiledPkgs[index].SHA1).To(Equal(expectedSHA))
	Expect(compiledManifest.CompiledPkgs[index].Dependencies).To(Equal(sourceManifest.Packages[index].Dependencies))

	Expect(compiledManifest.CompiledPkgs[index].Version).To(Equal(compiledManifest.CompiledPkgs[index].Fingerprint), "this seems to be equal on releases exported from a director")
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close() //nolint:errcheck
}

type writeTarballFileFunc func(tw *tar.Writer) error

func createTGZ(functions ...writeTarballFileFunc) ([]byte, error) {
	fn := func(w io.Writer) error {
		gw := gzip.NewWriter(w)
		defer closeAndIgnoreError(gw)
		tw := tar.NewWriter(gw)
		defer closeAndIgnoreError(tw)
		for _, fn := range functions {
			if err := fn(tw); err != nil {
				return err
			}
		}
		return nil
	}
	w := new(bytes.Buffer)
	err := fn(w)
	return w.Bytes(), err
}

func simpleFile(name string, content []byte, mode int64) writeTarballFileFunc {
	return func(tw *tar.Writer) error {
		if err := tw.WriteHeader(newSimpleFileHeader(name, int64(len(content)), mode)); err != nil {
			return err
		}
		_, err := tw.Write(content)
		if err != nil {
			return err
		}
		return nil
	}
}

func newSimpleFileHeader(name string, length, mode int64) *tar.Header {
	t := time.Unix(0, 0).In(time.UTC)
	return &tar.Header{
		Typeflag:   tar.TypeReg,
		Name:       name,
		Size:       length,
		ModTime:    t,
		AccessTime: t,
		ChangeTime: t,
		Mode:       mode,
	}
}

func listFileNamesInTarball(t GinkgoTInterface, filePath string) []tar.Header {
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
		_, _ = io.Copy(io.Discard, tr) //nolint:errcheck
	}
	return infos
}

func fakeCompilation(d directories.Provider) func(c compiler.Package, packages []models.Package) (string, boshcrypto.Digest, error) {
	return func(c compiler.Package, packages []models.Package) (string, boshcrypto.Digest, error) {
		blobContent, err := createTGZ(simpleFile("packaging", fmt.Appendf(nil, `"echo Compiled %q`, c.Name), 0o0744))
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

var _ = Describe("shippedPackages", func() {
	It("discovers packages from the job manifests in the log-cache release", func() {
		path := filepath.Join("testdata", "log-cache-release-3.0.9.tgz")
		manifest, err := releasetarball.Manifest(path)
		Expect(err).NotTo(HaveOccurred())

		shipped, err := releasetarball.ShippedPackagesForTest(path, manifest)
		Expect(err).NotTo(HaveOccurred())
		Expect(shipped).To(Equal(map[string]bool{
			"log-cache":               true,
			"log-cache-cf-auth-proxy": true,
			"log-cache-gateway":       true,
			"log-cache-syslog-server": true,
		}))
	})

	It("uses job.MF packages even when release.MF has an empty job package list", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}},
			Jobs:     []manifest.JobRef{{Name: "job", Packages: []string{}}},
		}, map[string]string{"job": "name: job\npackages: [pkg]\n"}, false)

		shipped, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).NotTo(HaveOccurred())
		Expect(shipped).To(Equal(map[string]bool{"pkg": true}))
	})

	It("accepts ./ prefixes in outer and inner archive names", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}},
			Jobs:     []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "name: job\npackages: [pkg]\n"}, true)

		shipped, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).NotTo(HaveOccurred())
		Expect(shipped).To(Equal(map[string]bool{"pkg": true}))
	})

	It("reads job.MF from an uncompressed tar archive", func() {
		path := writePlainTarJobRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, "name: job\npackages: [pkg]\n")
		shipped, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).NotTo(HaveOccurred())
		Expect(shipped).To(Equal(map[string]bool{"pkg": true}))
	})

	It("returns a YAML error for a malformed job.MF", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "packages: [unterminated"}, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("job")))
	})

	It("returns an error when a nested job tar cannot be read", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "invalid-archive"}, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("job")))
	})

	It("includes packages from a job archive not listed in release.MF", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}},
		}, map[string]string{"extra": "name: extra\npackages: [pkg]\n"}, false)
		shipped, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).NotTo(HaveOccurred())
		Expect(shipped).To(Equal(map[string]bool{"pkg": true}))
	})

	It("returns an error naming the job when its gzip header is truncated", func() {
		path := writeRawJobRelease(GinkgoT(), manifest.Manifest{Packages: []manifest.PackageRef{{Name: "pkg"}}}, "x", []byte{0x1f, 0x8b, 0x08, 0x00})
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring(`job "x"`)))
	})

	It("returns an error naming the job when gzip tar job.MF contents are truncated", func() {
		var inner bytes.Buffer
		gz := gzip.NewWriter(&inner)
		tarWriter := tar.NewWriter(gz)
		Expect(tarWriter.WriteHeader(&tar.Header{Name: "job.MF", Mode: 0o644, Size: 100})).To(Succeed())
		_, err := tarWriter.Write([]byte("name: x\n"))
		Expect(err).NotTo(HaveOccurred())
		Expect(gz.Close()).To(Succeed())

		path := writeRawJobRelease(GinkgoT(), manifest.Manifest{Packages: []manifest.PackageRef{{Name: "pkg"}}}, "x", inner.Bytes())
		_, err = releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring(`job "x"`)))
	})

	It("errors when a release job archive is missing", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "missing"}},
		}, nil, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("missing")))
	})

	It("errors when a job archive has no job.MF", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "missing"}, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("job")))
	})

	It("rejects duplicate job.MF entries", func() {
		for _, names := range [][]string{{"job.MF", "job.MF"}, {"job.MF", "./job.MF"}} {
			inner := createTar(
				simpleFile(names[0], []byte("name: job\npackages: [pkg]\n"), 0o644),
				simpleFile(names[1], []byte("name: job\npackages: [pkg]\n"), 0o644),
			)
			path := writeRawJobRelease(GinkgoT(), manifest.Manifest{
				Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
			}, "job", inner)
			_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
			Expect(err).To(MatchError(ContainSubstring("duplicate job.MF entries")))
		}
	})

	It("rejects a non-regular job.MF entry", func() {
		inner := createTar(func(tw *tar.Writer) error {
			return tw.WriteHeader(&tar.Header{Name: "job.MF", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "target"})
		})
		path := writeRawJobRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, "job", inner)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("job.MF is not a regular file")))
	})

	It("errors when a job manifest names a package outside the release manifest", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "known"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "name: job\npackages: [unknown]\n"}, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("unknown")))
	})

	It("errors when package releases have no shipped packages", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Packages: []manifest.PackageRef{{Name: "pkg"}}, Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "name: job\npackages: []\n"}, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("shipped packages")))
	})

	It("returns an empty set when the release has no packages and no jobs", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{}, nil, false)
		shipped, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).NotTo(HaveOccurred())
		Expect(shipped).To(BeEmpty())
	})

	It("validates jobs for zero-package releases", func() {
		path := writeShippedRelease(GinkgoT(), manifest.Manifest{
			Jobs: []manifest.JobRef{{Name: "job"}},
		}, map[string]string{"job": "name: job\npackages: [unknown]\n"}, false)
		_, err := releasetarball.ShippedPackagesForTest(path, mustReleaseManifest(path))
		Expect(err).To(MatchError(ContainSubstring("unknown package")))
	})
})

func writePlainTarJobRelease(t GinkgoTInterface, release manifest.Manifest, jobMF string) string {
	GinkgoHelper()
	releaseMF, err := yaml.Marshal(release)
	Expect(err).NotTo(HaveOccurred())
	inner := createTar(simpleFile("job.MF", []byte(jobMF), 0o644))
	archive, err := createTGZ(simpleFile("release.MF", releaseMF, 0o644), simpleFile("jobs/job.tgz", inner, 0o644))
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func writeRawJobRelease(t GinkgoTInterface, release manifest.Manifest, jobName string, inner []byte) string {
	GinkgoHelper()
	releaseMF, err := yaml.Marshal(release)
	Expect(err).NotTo(HaveOccurred())
	archive, err := createTGZ(simpleFile("release.MF", releaseMF, 0o644), simpleFile("jobs/"+jobName+".tgz", inner, 0o644))
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func createTar(functions ...writeTarballFileFunc) []byte {
	var buffer bytes.Buffer
	tw := tar.NewWriter(&buffer)
	for _, fn := range functions {
		Expect(fn(tw)).To(Succeed())
	}
	Expect(tw.Close()).To(Succeed())
	return buffer.Bytes()
}

func writeShippedRelease(t GinkgoTInterface, release manifest.Manifest, jobs map[string]string, dotPrefix bool) string {
	releaseMF, err := yaml.Marshal(release)
	Expect(err).NotTo(HaveOccurred())
	files := []writeTarballFileFunc{simpleFile("release.MF", releaseMF, 0o644)}
	for name, jobMF := range jobs {
		innerName := "job.MF"
		outerName := "jobs/" + name + ".tgz"
		if dotPrefix {
			innerName = "./" + innerName
			outerName = "./" + outerName
		}
		var inner []byte
		switch jobMF {
		case "invalid-archive":
			inner = []byte("invalid nested tar")
		case "missing":
			inner, err = createTGZ()
		default:
			inner, err = createTGZ(simpleFile(innerName, []byte(jobMF), 0o644))
		}
		Expect(err).NotTo(HaveOccurred())
		files = append(files, simpleFile(outerName, inner, 0o644))
	}
	archive, err := createTGZ(files...)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func mustReleaseManifest(path string) manifest.Manifest {
	result, err := releasetarball.Manifest(path)
	Expect(err).NotTo(HaveOccurred())
	return result
}

func rewriteReleaseManifest(t GinkgoTInterface, source string, replacement manifest.Manifest) string {
	GinkgoHelper()
	f, err := os.Open(source)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(f)
	gr, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(gr)
	tr := tar.NewReader(gr)
	var files []writeTarballFileFunc
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		if h.FileInfo().IsDir() {
			continue
		}
		contents, err := io.ReadAll(tr)
		Expect(err).NotTo(HaveOccurred())
		if filepath.Base(h.Name) == "release.MF" {
			contents, err = yaml.Marshal(replacement)
			Expect(err).NotTo(HaveOccurred())
		}
		files = append(files, simpleFile(h.Name, contents, h.Mode))
	}
	archive, err := createTGZ(files...)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func rewriteJobPackages(t GinkgoTInterface, source string, packageNames []string) string {
	GinkgoHelper()
	f, err := os.Open(source)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(f)
	gr, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(gr)
	tr := tar.NewReader(gr)
	var files []writeTarballFileFunc
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		if h.FileInfo().IsDir() {
			continue
		}
		contents, err := io.ReadAll(tr)
		Expect(err).NotTo(HaveOccurred())
		if filepath.Base(filepath.Dir(h.Name)) == "jobs" && filepath.Ext(h.Name) == ".tgz" {
			jobName := strings.TrimSuffix(filepath.Base(h.Name), ".tgz")
			jobContents := fmt.Sprintf("name: %s\npackages: [%s]\n", jobName, strings.Join(packageNames, ", "))
			contents, err = createTGZ(simpleFile("job.MF", []byte(jobContents), 0o644))
			Expect(err).NotTo(HaveOccurred())
		}
		files = append(files, simpleFile(h.Name, contents, h.Mode))
	}
	archive, err := createTGZ(files...)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func removeJobArchives(t GinkgoTInterface, source string) string {
	return rewriteJobArchives(t, source, true)
}

func replaceJobArchives(t GinkgoTInterface, source string) string {
	return rewriteJobArchives(t, source, false)
}

func rewriteJobArchives(t GinkgoTInterface, source string, remove bool) string {
	GinkgoHelper()
	f, err := os.Open(source)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(f)
	gr, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(gr)
	tr := tar.NewReader(gr)
	var files []writeTarballFileFunc
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		if h.FileInfo().IsDir() {
			continue
		}
		contents, err := io.ReadAll(tr)
		Expect(err).NotTo(HaveOccurred())
		if filepath.Base(filepath.Dir(h.Name)) == "jobs" && filepath.Ext(h.Name) == ".tgz" {
			if remove {
				continue
			}
			contents, err = createTGZ()
			Expect(err).NotTo(HaveOccurred())
		}
		files = append(files, simpleFile(h.Name, contents, h.Mode))
	}
	archive, err := createTGZ(files...)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func releaseManifestBytes(filePath string) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer closeAndIgnoreError(f)
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer closeAndIgnoreError(gr)
	tr := tar.NewReader(gr)
	for {
		h, err := tr.Next()
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == "release.MF" {
			return io.ReadAll(tr)
		}
	}
}
