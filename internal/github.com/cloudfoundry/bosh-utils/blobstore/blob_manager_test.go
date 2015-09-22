package blobstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/blobstore"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
)

func createBlobManager() (blobManager BlobManager, fs boshsys.FileSystem) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	fs = boshsys.NewOsFileSystem(logger)
	blobManager = NewBlobManager(fs, "/tmp")
	return
}

func readFile(fileIO boshsys.File) (fileBytes []byte) {
	fileStat, _ := fileIO.Stat()
	fileBytes = make([]byte, fileStat.Size())
	fileIO.Read(fileBytes)
	return
}

var _ = Describe("Testing with Ginkgo", func() {
	It("fetch", func() {
		blobManager, fs := createBlobManager()
		fs.WriteFileString("/tmp/105d33ae-655c-493d-bf9f-1df5cf3ca847", "some data")

		readOnlyFile, err := blobManager.Fetch("105d33ae-655c-493d-bf9f-1df5cf3ca847")
		defer fs.RemoveAll(readOnlyFile.Name())

		Expect(err).ToNot(HaveOccurred())
		fileBytes := readFile(readOnlyFile)

		Expect(string(fileBytes)).To(Equal("some data"))
	})

	It("write", func() {

		blobManager, fs := createBlobManager()
		fs.WriteFileString("/tmp/105d33ae-655c-493d-bf9f-1df5cf3ca847", "some data")
		defer fs.RemoveAll("/tmp/105d33ae-655c-493d-bf9f-1df5cf3ca847")

		err := blobManager.Write("105d33ae-655c-493d-bf9f-1df5cf3ca847", []byte("new data"))
		Expect(err).ToNot(HaveOccurred())

		contents, err := fs.ReadFileString("/tmp/105d33ae-655c-493d-bf9f-1df5cf3ca847")
		Expect(err).ToNot(HaveOccurred())
		Expect(contents).To(Equal("new data"))
	})
})
