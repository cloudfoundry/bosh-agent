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
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	"gopkg.in/yaml.v3"

	"github.com/cloudfoundry/bosh-agent/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/agent/compiler"
	"github.com/cloudfoundry/bosh-agent/releasetarball"
	"github.com/cloudfoundry/bosh-agent/releasetarball/internal/fakes"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
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

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -fake-name Compiler -o internal/fakes/compiler.go github.com/cloudfoundry/bosh-agent/agent/compiler.Compiler

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
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
			Expect(err).NotTo(HaveOccurred())
			Expect(pkgCompiler.CompileCallCount()).To(BeZero())
		})
	})

	When("compiling a tarball with a package dependency cycle", func() {
		BeforeEach(func() {
			releaseInputDir := GinkgoT().TempDir()
			sourceTarballPath = filepath.Join(releaseInputDir, "banana.tgz")

			releaseMF, _ := yaml.Marshal(manifest.Manifest{
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
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
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
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
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
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
			Expect(err).To(MatchError(ContainSubstring("banana")))
		})
	})

	When("package tarball is not found in release manifest", func() {
		BeforeEach(func() {
			releaseInputDir := GinkgoT().TempDir()
			sourceTarballPath = filepath.Join(releaseInputDir, "banana.tgz")
			releaseMF, _ := yaml.Marshal(manifest.Manifest{})

			tgz, err := createTGZ(
				simpleFile("release.MF", releaseMF, 0o0644),
				simpleFile("packages/a.tgz", nil, 0o0644),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(sourceTarballPath, tgz, 0o0644)).To(Succeed())
		})

		It("returns a helpful error", func() {
			const stemcellSlug = "banana-slug/1.23"
			_, err := releasetarball.Compile(pkgCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
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
			resultPath, err := releasetarball.Compile(multiplePackageCompiler, sourceTarballPath, d.BlobsDir(), releasesOutputDir, stemcellSlug)
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
	Expect(compiledManifest.CompiledPkgs[index].Name).To(Equal(sourceManifest.Packages[index].Name))
	Expect(compiledManifest.CompiledPkgs[index].Version).To(Equal(sourceManifest.Packages[index].Version))
	Expect(compiledManifest.CompiledPkgs[index].Fingerprint).To(Equal(sourceManifest.Packages[index].Fingerprint))
	Expect(compiledManifest.CompiledPkgs[index].OSVersionSlug).To(Equal(stemcellSlug))
	Expect(compiledManifest.CompiledPkgs[index].SHA1).To(Equal(expectedSHA))
	Expect(compiledManifest.CompiledPkgs[index].Dependencies).To(Equal(sourceManifest.Packages[index].Dependencies))

	Expect(compiledManifest.CompiledPkgs[index].Version).To(Equal(compiledManifest.CompiledPkgs[index].Fingerprint), "this seems to be equal on releases exported from a director")
}

func closeAndIgnoreError(c io.Closer) {
	_ = c.Close()
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
		_, _ = io.Copy(io.Discard, tr)
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
