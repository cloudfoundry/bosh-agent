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
	"time"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/v2/agent/compiler"
	"github.com/cloudfoundry/bosh-agent/v2/releasetarball"
	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

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

func writePlainTarJobRelease(t GinkgoTInterface, release manifest.Manifest, jobMF string) string {
	return writeRawJobRelease(t, release, "job", createTar(simpleFile("job.MF", []byte(jobMF), 0o644)))
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

// rewriteReleaseArchive keeps archive setup in one place for malformed-input tests.
func rewriteReleaseArchive(t GinkgoTInterface, source string, rewrite func(string, []byte) ([]byte, bool)) string {
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
		if contents, keep := rewrite(h.Name, contents); keep {
			files = append(files, simpleFile(h.Name, contents, h.Mode))
		}
	}
	archive, err := createTGZ(files...)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(t.TempDir(), "release.tgz")
	Expect(os.WriteFile(path, archive, 0o644)).To(Succeed())
	return path
}

func rewriteReleaseManifest(t GinkgoTInterface, source string, replacement manifest.Manifest) string {
	return rewriteReleaseArchive(t, source, func(name string, contents []byte) ([]byte, bool) {
		if filepath.Base(name) == "release.MF" {
			var err error
			contents, err = yaml.Marshal(replacement)
			Expect(err).NotTo(HaveOccurred())
		}
		return contents, true
	})
}

func rewriteJobPackages(t GinkgoTInterface, source string, packageNames []string) string {
	return rewriteReleaseArchive(t, source, func(name string, contents []byte) ([]byte, bool) {
		if filepath.Base(filepath.Dir(name)) == "jobs" && filepath.Ext(name) == ".tgz" {
			jobName := strings.TrimSuffix(filepath.Base(name), ".tgz")
			jobMF := fmt.Sprintf("name: %s\npackages: [%s]\n", jobName, strings.Join(packageNames, ", "))
			var err error
			contents, err = createTGZ(simpleFile("job.MF", []byte(jobMF), 0o644))
			Expect(err).NotTo(HaveOccurred())
		}
		return contents, true
	})
}

func removeJobArchives(t GinkgoTInterface, source string) string {
	return rewriteJobArchives(t, source, true)
}

func replaceJobArchives(t GinkgoTInterface, source string) string {
	return rewriteJobArchives(t, source, false)
}

func rewriteJobArchives(t GinkgoTInterface, source string, remove bool) string {
	return rewriteReleaseArchive(t, source, func(name string, contents []byte) ([]byte, bool) {
		if filepath.Base(filepath.Dir(name)) == "jobs" && filepath.Ext(name) == ".tgz" {
			if remove {
				return nil, false
			}
			var err error
			contents, err = createTGZ()
			Expect(err).NotTo(HaveOccurred())
		}
		return contents, true
	})
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
