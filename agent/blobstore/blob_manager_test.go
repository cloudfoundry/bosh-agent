package blobstore_test

import (
	"io/ioutil"
	"os"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore"
)

var _ = Describe("Blob Manager", func() {
	var (
		fs       boshsys.FileSystem
		basePath string

		blobManager boshagentblobstore.BlobManagerInterface
	)

	blobID := "blob-id"

	BeforeEach(func() {
		var err error
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fs = boshsys.NewOsFileSystem(logger)
		basePath, err = ioutil.TempDir("", "blobmanager")
		Expect(err).NotTo(HaveOccurred())

		blobManager, err = boshagentblobstore.NewBlobManager(basePath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.Chmod(basePath, 0700)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(basePath)
		Expect(err).NotTo(HaveOccurred())
	})

	getBlob := func(id string) string {
		file, status, err := blobManager.Fetch(id)
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()
		Expect(status).To(Equal(200))

		contents, err := fs.ReadFileString(file.Name())
		Expect(err).ToNot(HaveOccurred())

		return contents
	}

	It("can fetch what was written", func() {
		err := blobManager.Write(blobID, strings.NewReader("new data"))
		Expect(err).ToNot(HaveOccurred())

		contents := getBlob(blobID)
		Expect(contents).To(Equal("new data"))
	})

	It("can overwrite files", func() {
		err := blobManager.Write(blobID, strings.NewReader("old data"))
		Expect(err).ToNot(HaveOccurred())

		err = blobManager.Write(blobID, strings.NewReader("new data"))
		Expect(err).ToNot(HaveOccurred())

		contents := getBlob(blobID)
		Expect(contents).To(Equal("new data"))
	})

	It("can store multiple files", func() {
		err := blobManager.Write(blobID, strings.NewReader("data1"))
		Expect(err).ToNot(HaveOccurred())

		otherBlobID := "other-blob-id"
		err = blobManager.Write(otherBlobID, strings.NewReader("data2"))
		Expect(err).ToNot(HaveOccurred())

		contents := getBlob(blobID)
		Expect(contents).To(Equal("data1"))

		otherContents := getBlob(otherBlobID)
		Expect(otherContents).To(Equal("data2"))
	})

	It("reports invalid permissions as an error", func() {
		// Windows file reads, unlike Unix, are not affected by the permissions of
		// parent directories. This means we cannot modify the permissions of the
		// blobs in a blackbox way.
		if runtime.GOOS == "windows" {
			Skip("Chmod() implementation on Windows has different semantics")
		}

		err := blobManager.Write(blobID, strings.NewReader("data1"))
		Expect(err).ToNot(HaveOccurred())

		err = os.Chmod(basePath, 0000)
		Expect(err).ToNot(HaveOccurred())

		_, status, err := blobManager.Fetch(blobID)
		Expect(err).To(HaveOccurred())
		Expect(status).To(Equal(500))
	})

	It("is compatible with the local blobstore when overlaid on the same directory (used for bosh create-env)", func() {
		err := blobManager.Write(blobID, strings.NewReader("data"))
		Expect(err).ToNot(HaveOccurred())

		blobstore := boshblob.NewLocalBlobstore(fs, nil, map[string]interface{}{
			"blobstore_path": basePath,
		})

		blob, err := blobstore.Get(blobID)
		Expect(err).ToNot(HaveOccurred())

		bs, err := ioutil.ReadFile(blob)
		Expect(err).ToNot(HaveOccurred())
		Expect(bs).To(Equal([]byte("data")))
	})

	Describe("GetPath", func() {
		var sampleDigest boshcrypto.Digest

		BeforeEach(func() {
			correctCheckSum := "a17c9aaa61e80a1bf71d0d850af4e5baa9800bbd" // sha-1 of "data"
			sampleDigest = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, correctCheckSum)
		})

		Context("when file requested exists in blobsPath", func() {
			BeforeEach(func() {
				err := blobManager.Write(blobID, strings.NewReader("data"))
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when file checksum matches provided checksum", func() {
				It("should return the path of a copy of the requested blob", func() {
					filename, err := blobManager.GetPath(blobID, sampleDigest)
					Expect(err).NotTo(HaveOccurred())

					contents, err := fs.ReadFileString(filename)
					Expect(err).NotTo(HaveOccurred())
					Expect(contents).To(Equal("data"))
				})
			})

			Context("when file checksum does NOT match provided checksum", func() {
				It("should return an error", func() {
					bogusDigest := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "bogus-sha")
					_, err := blobManager.GetPath(blobID, bogusDigest)
					Expect(err).To(MatchError(ContainSubstring(blobID)))
					Expect(err).To(MatchError(ContainSubstring(sampleDigest.String())))
					Expect(err).To(MatchError(ContainSubstring(bogusDigest.String())))
				})
			})

			It("does not allow modifications made to the returned path to affect the original file", func() {
				path, err := blobManager.GetPath(blobID, sampleDigest)
				Expect(err).NotTo(HaveOccurred())

				err = fs.WriteFileString(path, "overwriting!")
				Expect(err).NotTo(HaveOccurred())

				contents := getBlob(blobID)
				Expect(contents).To(Equal("data"))
			})

			It("puts the temporary file inside the work directory to make sure that no files leak out if it's mounted on a tmpfs", func() {
				path, err := blobManager.GetPath(blobID, sampleDigest)
				Expect(err).NotTo(HaveOccurred())

				Expect(path).To(HavePrefix(basePath))
			})
		})

		Context("when file requested does not exist in blobsPath", func() {
			It("returns an error", func() {
				_, err := blobManager.GetPath("missing", sampleDigest)
				Expect(err).To(MatchError("Blob 'missing' not found"))
			})
		})
	})

	Describe("Delete", func() {
		Context("when file to be deleted exists in blobsPath", func() {
			BeforeEach(func() {
				err := blobManager.Write(blobID, strings.NewReader("data"))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should delete the blob", func() {
				exists := blobManager.BlobExists(blobID)
				Expect(exists).To(BeTrue())

				err := blobManager.Delete(blobID)
				Expect(err).NotTo(HaveOccurred())

				exists = blobManager.BlobExists(blobID)
				Expect(exists).To(BeFalse())

				_, status, err := blobManager.Fetch(blobID)
				Expect(err).To(HaveOccurred())
				Expect(status).To(Equal(404))
			})
		})

		Context("when file to be deleted does not exist in blobsPath", func() {
			It("does not error", func() {
				err := blobManager.Delete("hello-i-am-no-one")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("BlobExists", func() {
		Context("when blob requested exists in blobsPath", func() {
			BeforeEach(func() {
				err := blobManager.Write(blobID, strings.NewReader("super-smurf-content"))
				Expect(err).To(BeNil())
			})

			It("returns true", func() {
				exists := blobManager.BlobExists(blobID)
				Expect(exists).To(BeTrue())
			})
		})

		Context("when blob requested does NOT exist in blobsPath", func() {
			It("returns false", func() {
				exists := blobManager.BlobExists("blob-id-does-not-exist")
				Expect(exists).To(BeFalse())
			})
		})
	})
})
