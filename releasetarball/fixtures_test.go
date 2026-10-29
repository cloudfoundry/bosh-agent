//go:build fixtures

package releasetarball_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/releasetarball"
	"github.com/cloudfoundry/bosh-agent/v2/releasetarball/internal/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Compile fixtures", func() {
	DescribeTable("preserves compiled blobs and trims only build dependencies", func(name string) {
		fixtureDir := os.Getenv("FIXTURES")
		if fixtureDir == "" {
			fixtureDir = "testdata"
		}
		fixture := filepath.Join(fixtureDir, name)
		input := prepareFixture(GinkgoT(), fixture)
		inputManifest := mustReleaseManifest(input)
		shipped := jobPackageNames(GinkgoT(), input)
		allPackages := packageNames(inputManifest.Packages)
		kept := shipped
		dropped := difference(allPackages, shipped)

		workDir := GinkgoT().TempDir()
		plainDir := filepath.Join(workDir, "plain")
		flaggedDir := filepath.Join(workDir, "flagged")
		blobs := directories.NewProvider(filepath.Join(workDir, "blobs"))
		Expect(os.MkdirAll(blobs.BlobsDir(), 0o755)).To(Succeed())
		Expect(os.MkdirAll(plainDir, 0o755)).To(Succeed())
		Expect(os.MkdirAll(flaggedDir, 0o755)).To(Succeed())
		pkgCompiler := new(fakes.Compiler)
		pkgCompiler.CompileCalls(fakeCompilation(blobs))

		plainPath, plain := compileFixture(pkgCompiler, input, blobs.BlobsDir(), plainDir, false)
		flaggedPath, flagged := compileFixture(pkgCompiler, input, blobs.BlobsDir(), flaggedDir, true)
		Expect(packageNamesCompiled(flagged.CompiledPkgs)).To(Equal(kept))
		Expect(packageNamesCompileTime(flagged.CompileTimePackages)).To(Equal(dropped))
		assertCompiledBlobs(flaggedPath, flagged.CompiledPkgs)
		assertCompiledBlobs(plainPath, plain.CompiledPkgs)
		assertKeptPackages(inputManifest, flagged, kept)

		if len(dropped) == 0 {
			plainBytes, err := os.ReadFile(plainPath)
			Expect(err).NotTo(HaveOccurred())
			flaggedBytes, err := os.ReadFile(flaggedPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(sha256.Sum256(flaggedBytes)).To(Equal(sha256.Sum256(plainBytes)))
		} else {
			wantVersion := inputManifest.Version + "+trimmed"
			if strings.Contains(inputManifest.Version, "+") {
				wantVersion = inputManifest.Version + ".trimmed"
			}
			Expect(flagged.Version).To(Equal(wantVersion))
		}

		copyFixtureOutputs(fixture, flaggedPath, plainPath)
		GinkgoWriter.Printf("fixture %s: flagged=%s plain=%s kept=%d dropped=%v\n", filepath.Base(fixture), flaggedPath, plainPath, len(kept), dropped)
	},
		Entry("source release", "log-cache-release-3.0.9.tgz"),
		Entry("compiled release", "log-cache-3.0.9-banana-slug-1.23.tgz"),
	)
})

func prepareFixture(t GinkgoTInterface, fixture string) string {
	GinkgoHelper()
	dir := t.TempDir()
	extractFixture(t, fixture, dir)
	manifestPath := filepath.Join(dir, "release.MF")
	raw, err := os.ReadFile(manifestPath)
	Expect(err).NotTo(HaveOccurred())
	var release manifest.Manifest
	Expect(yaml.Unmarshal(raw, &release)).To(Succeed())
	if len(release.CompiledPkgs) > 0 && len(release.Packages) == 0 {
		var doc map[string]any
		Expect(yaml.Unmarshal(raw, &doc)).To(Succeed())
		for _, pkg := range release.CompiledPkgs {
			release.Packages = append(release.Packages, manifest.PackageRef{
				Name: pkg.Name, Version: pkg.Version, Fingerprint: pkg.Fingerprint, SHA1: pkg.SHA1, Dependencies: slices.Clone(pkg.Dependencies),
			})
			oldBlob := filepath.Join(dir, "compiled_packages", pkg.Name+".tgz")
			newDir := filepath.Join(dir, "packages")
			Expect(os.MkdirAll(newDir, 0o755)).To(Succeed())
			Expect(os.Rename(oldBlob, filepath.Join(newDir, pkg.Name+".tgz"))).To(Succeed())
		}
		delete(doc, "compiled_packages")
		packages := make([]any, 0, len(release.Packages))
		for _, pkg := range release.Packages {
			packages = append(packages, map[string]any{
				"name": pkg.Name, "version": pkg.Version, "fingerprint": pkg.Fingerprint,
				"sha1": pkg.SHA1, "dependencies": pkg.Dependencies,
			})
		}
		doc["packages"] = packages
		converted, err := yaml.Marshal(doc)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(manifestPath, converted, 0o644)).To(Succeed())
	}
	return createTarballFromDir(t, dir)
}

func extractFixture(t GinkgoTInterface, fixture, dir string) {
	GinkgoHelper()
	f, err := os.Open(fixture)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(f)
	gz, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(gz)
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		clean := filepath.Clean(filepath.FromSlash(h.Name))
		Expect(filepath.IsLocal(clean)).To(BeTrue(), "invalid tar path %q", h.Name)
		dst := filepath.Join(dir, clean)
		switch h.Typeflag {
		case tar.TypeDir:
			Expect(os.MkdirAll(dst, 0o755)).To(Succeed())
		case tar.TypeReg:
			Expect(os.MkdirAll(filepath.Dir(dst), 0o755)).To(Succeed())
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			Expect(err).NotTo(HaveOccurred())
			_, copyErr := io.Copy(out, tr)
			Expect(out.Close()).To(Succeed())
			Expect(copyErr).NotTo(HaveOccurred())
		default:
			Fail(fmt.Sprintf("fixture contains unsupported tar entry %q", h.Name))
		}
	}
}

func createTarballFromDir(t GinkgoTInterface, dir string) string {
	GinkgoHelper()
	var directories, files []string
	Expect(filepath.WalkDir(dir, func(p string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, filepath.ToSlash(rel))
		} else {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})).To(Succeed())
	slices.Sort(directories)
	slices.Sort(files)
	output := filepath.Join(t.TempDir(), "input.tgz")
	f, err := os.Create(output)
	Expect(err).NotTo(HaveOccurred())
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for _, name := range directories {
		Expect(tw.WriteHeader(&tar.Header{Name: name + "/", Mode: 0o755, Typeflag: tar.TypeDir})).To(Succeed())
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		Expect(err).NotTo(HaveOccurred())
		Expect(tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg})).To(Succeed())
		_, err = tw.Write(data)
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(tw.Close()).To(Succeed())
	Expect(gw.Close()).To(Succeed())
	Expect(f.Close()).To(Succeed())
	return output
}

func assertCompiledBlobs(tarball string, packages []manifest.CompiledPackageRef) {
	GinkgoHelper()
	f, err := os.Open(tarball)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(f)
	gz, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(gz)
	tr := tar.NewReader(gz)
	entries := map[string][][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		contents, err := io.ReadAll(tr)
		Expect(err).NotTo(HaveOccurred())
		name := strings.TrimPrefix(h.Name, "./")
		entries[name] = append(entries[name], contents)
	}
	var names []string
	for name := range entries {
		if strings.HasPrefix(name, "compiled_packages/") && strings.HasSuffix(name, ".tgz") {
			names = append(names, name)
		}
	}
	expected := make([]string, 0, len(packages))
	for _, pkg := range packages {
		expected = append(expected, "compiled_packages/"+pkg.Name+".tgz")
	}
	Expect(names).To(ConsistOf(expected), "archive must contain exactly the manifest's compiled blobs")
	for _, pkg := range packages {
		name := "compiled_packages/" + pkg.Name + ".tgz"
		contents := entries[name]
		Expect(contents).To(HaveLen(1), name)
		if strings.HasPrefix(pkg.SHA1, "sha256:") {
			actual := sha256.Sum256(contents[0])
			Expect("sha256:"+hex.EncodeToString(actual[:])).To(Equal(pkg.SHA1), name)
		} else {
			actual := sha1.Sum(contents[0])
			Expect(hex.EncodeToString(actual[:])).To(Equal(pkg.SHA1), name)
		}
	}
}

func compileFixture(pkgCompiler *fakes.Compiler, input, blobsDir, outputDir string, exclude bool) (string, manifest.Manifest) {
	GinkgoHelper()
	output, err := releasetarball.Compile(pkgCompiler, input, blobsDir, outputDir, "banana-slug/1.23", releasetarball.CompileOptions{ExcludeCompileTimePackages: exclude})
	Expect(err).NotTo(HaveOccurred())
	return output, mustReleaseManifest(output)
}

type jobManifest struct {
	Packages []string `yaml:"packages"`
}

func jobPackageNames(t GinkgoTInterface, input string) []string {
	GinkgoHelper()
	f, err := os.Open(input)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(f)
	gz, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer closeAndIgnoreError(gz)
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		Expect(err).NotTo(HaveOccurred())
		if !strings.HasPrefix(h.Name, "jobs/") || !strings.HasSuffix(h.Name, ".tgz") {
			continue
		}
		jobBytes, err := io.ReadAll(tr)
		Expect(err).NotTo(HaveOccurred())
		jobGz, err := gzip.NewReader(bytes.NewReader(jobBytes))
		Expect(err).NotTo(HaveOccurred())
		jobTar := tar.NewReader(jobGz)
		for {
			jobHeader, err := jobTar.Next()
			if err == io.EOF {
				break
			}
			Expect(err).NotTo(HaveOccurred())
			if strings.TrimPrefix(jobHeader.Name, "./") != "job.MF" {
				continue
			}
			var job jobManifest
			Expect(yaml.NewDecoder(jobTar).Decode(&job)).To(Succeed())
			names = append(names, job.Packages...)
		}
		Expect(jobGz.Close()).To(Succeed())
	}
	return uniqueSorted(names)
}

func assertKeptPackages(source, output manifest.Manifest, kept []string) {
	GinkgoHelper()
	sourcePackages := map[string]manifest.PackageRef{}
	for _, pkg := range source.Packages {
		sourcePackages[pkg.Name] = pkg
	}
	keptSet := make(map[string]bool, len(kept))
	for _, name := range kept {
		keptSet[name] = true
	}
	for _, compiled := range output.CompiledPkgs {
		original := sourcePackages[compiled.Name]
		var dependencies []string
		for _, dependency := range original.Dependencies {
			if keptSet[dependency] {
				dependencies = append(dependencies, dependency)
			}
		}
		Expect(slices.Equal(compiled.Dependencies, dependencies)).To(BeTrue(), compiled.Name)
		if dependencyClosureContainsDropped(original.Name, sourcePackages, keptSet, map[string]bool{}) {
			Expect(compiled.OriginalFingerprint).NotTo(BeEmpty(), compiled.Name)
			Expect(compiled.OriginalFingerprint).NotTo(Equal(compiled.Fingerprint), compiled.Name)
			Expect(compiled.Version).To(Equal(compiled.Fingerprint), compiled.Name)
		} else {
			Expect(compiled.OriginalFingerprint).To(BeEmpty(), compiled.Name)
			Expect(compiled.Version).To(Equal(original.Version), compiled.Name)
			Expect(compiled.Fingerprint).To(Equal(original.Fingerprint), compiled.Name)
		}
	}
}

func dependencyClosureContainsDropped(name string, packages map[string]manifest.PackageRef, kept, visited map[string]bool) bool {
	if visited[name] {
		return false
	}
	visited[name] = true
	for _, dependency := range packages[name].Dependencies {
		if !kept[dependency] || dependencyClosureContainsDropped(dependency, packages, kept, visited) {
			return true
		}
	}
	return false
}

func copyFixtureOutputs(fixture, flagged, plain string) {
	GinkgoHelper()
	outputDir := os.Getenv("FIXTURES_OUT")
	if outputDir == "" {
		return
	}
	Expect(os.MkdirAll(outputDir, 0o755)).To(Succeed())
	base := filepath.Base(fixture)
	for _, output := range []struct{ source, suffix string }{{flagged, ".flagged.tgz"}, {plain, ".plain.tgz"}} {
		data, err := os.ReadFile(output.source)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(outputDir, strings.TrimSuffix(base, ".tgz")+output.suffix), data, 0o644)).To(Succeed())
	}
}

func packageNames(packages []manifest.PackageRef) []string {
	names := make([]string, 0, len(packages))
	for _, pkg := range packages {
		names = append(names, pkg.Name)
	}
	return uniqueSorted(names)
}

func packageNamesCompiled(packages []manifest.CompiledPackageRef) []string {
	names := make([]string, 0, len(packages))
	for _, pkg := range packages {
		names = append(names, pkg.Name)
	}
	return uniqueSorted(names)
}

func packageNamesCompileTime(packages []manifest.CompileTimePackageRef) []string {
	names := make([]string, 0, len(packages))
	for _, pkg := range packages {
		names = append(names, pkg.Name)
	}
	return uniqueSorted(names)
}

func uniqueSorted(names []string) []string {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	result := make([]string, 0, len(set))
	for name := range set {
		result = append(result, name)
	}
	slices.Sort(result)
	return result
}

func difference(left, right []string) []string {
	rightSet := map[string]bool{}
	for _, name := range right {
		rightSet[name] = true
	}
	var result []string
	for _, name := range left {
		if !rightSet[name] {
			result = append(result, name)
		}
	}
	return uniqueSorted(result)
}
