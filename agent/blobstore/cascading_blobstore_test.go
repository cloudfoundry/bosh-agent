package blobstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/blobstore"
	"github.com/cloudfoundry/bosh-agent/logger/fakes"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	fakeblob "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"io/ioutil"
	"path"
)

var _ = Describe("cascadingBlobstore", func() {
	var (
		fs                 boshsys.FileSystem
		innerBlobstore     *fakeblob.FakeBlobstore
		logger             *fakes.FakeLogger
		cascadingBlobstore boshblob.Blobstore
		blobsDir           string
	)

	BeforeEach(func() {
		logger = &fakes.FakeLogger{}
		fs = boshsys.NewOsFileSystem(logger)
		blobsDir, _ = ioutil.TempDir("", "vroom")

		innerBlobstore = &fakeblob.FakeBlobstore{}
		cascadingBlobstore = blobstore.NewCascadingBlobstore(fs, innerBlobstore, blobsDir)
	})

	Describe("Get", func() {
		Describe("when the requested file exists in blobsDir", func() {
			It("returns the name of the file on the file system", func() {
				blobID := "smurf-1"
				sha1 := "smurf-1-sha"

				err := fs.WriteFileString(path.Join(blobsDir, blobID), "smurf-content")
				Expect(err).To(BeNil())

				filename, err := cascadingBlobstore.Get(blobID, sha1)

				Expect(err).To(BeNil())
				Expect(fs.ReadFileString(filename)).To(Equal("smurf-content"))

				Expect(innerBlobstore.GetBlobIDs).Should(BeEmpty())
			})

			It("should return the path to a copy of the requested file", func() {
				blobID := "smurf-2"
				sha1 := "smurf-2-sha"

				err := fs.WriteFileString(path.Join(blobsDir, blobID), "smurf-content")
				Expect(err).To(BeNil())

				filename, err := cascadingBlobstore.Get(blobID, sha1)

				Expect(err).To(BeNil())
				Expect(fs.ReadFileString(filename)).To(Equal("smurf-content"))
				Expect(filename).ToNot(Equal(path.Join(blobsDir, blobID)))

				Expect(innerBlobstore.GetBlobIDs).Should(BeEmpty())
			})
		})

		Describe("when file requested does not exist in blobsDir", func() {
			It("delegates the action of getting the blob to inner blobstore", func() {
				blobID := "smurf-3"
				sha1 := "smurf-3-sha"

				_, err := cascadingBlobstore.Get(blobID, sha1)

				Expect(err).To(BeNil())
				Expect(innerBlobstore.GetBlobIDs).ShouldNot(BeEmpty())
			})

			It("returns the name of the file in the inner blobstore when it cannot find it on the local file system", func() {
				blobID := "smurf-4"
				sha1 := "smurf-4-sha"

				innerBlobstore.GetFileName = "/smurf-tmpfile"
				innerBlobstore.GetError = nil
				innerBlobstore.CreateBlobID = "createdBlobID"
				innerBlobstore.CreateFingerprint = "createdSha"

				resultFileName, err := cascadingBlobstore.Get(blobID, sha1)

				Expect(err).To(BeNil())

				Expect(len(innerBlobstore.GetBlobIDs)).To(Equal(1))
				Expect(len(innerBlobstore.GetFingerprints)).To(Equal(1))

				Expect(innerBlobstore.GetBlobIDs[0]).To(Equal(blobID))
				Expect(innerBlobstore.GetFingerprints[0]).To(Equal(sha1))

				Expect(resultFileName).To(Equal("/smurf-tmpfile"))
			})
		})

		It("returns an error if the blobID is not in blobsDir or the inner blobstore", func() {
			innerBlobstore.GetError = errors.New("error returned by inner blobstore")

			_, err := cascadingBlobstore.Get("whatever", "hiha")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error returned by inner blobstore"))
		})
	})

	Describe("CleanUp", func() {
		It("delegates the action to the inner blobstore", func() {
			innerBlobstore.CleanUpErr = nil

			err := cascadingBlobstore.CleanUp("fileToDelete")
			Expect(err).To(BeNil())
			Expect(innerBlobstore.CleanUpFileName).To(Equal("fileToDelete"))
		})

		It("returns an error if the inner blobstore fails to clean up", func() {
			innerBlobstore.CleanUpErr = errors.New("error cleaning up")

			err := cascadingBlobstore.CleanUp("randomFile")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error cleaning up"))
		})
	})

	Describe("Create", func() {
		It("delegates the action to the inner blobstore", func() {
			innerBlobstore.CreateErr = nil
			innerBlobstore.CreateBlobID = "createBlobId"
			innerBlobstore.CreateFingerprint = "createFingerprint"

			createdBlobID, createdFingerprint, err := cascadingBlobstore.Create("createdFile")

			Expect(err).To(BeNil())

			Expect(createdBlobID).To(Equal("createBlobId"))
			Expect(createdFingerprint).To(Equal("createFingerprint"))

			Expect(innerBlobstore.CreateFileNames).ShouldNot(BeEmpty())
			Expect(len(innerBlobstore.CreateFileNames)).To(Equal(1))
			Expect(innerBlobstore.CreateFileNames[0]).To(Equal("createdFile"))
		})

		It("returns an error if the inner blobstore fails to create", func() {
			innerBlobstore.CreateErr = errors.New("error creating")

			_, _, err := cascadingBlobstore.Create("createdFile")

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error creating"))
		})
	})

	Describe("Validate", func() {
		It("delegates the action to the inner blobstore", func() {
			err := cascadingBlobstore.Validate()

			Expect(err).To(BeNil())
		})

		It("returns an error if the inner blobstore fails to validate", func() {
			innerBlobstore.ValidateError = errors.New("error validating")

			err := cascadingBlobstore.Validate()

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error validating"))
		})
	})

	Describe("Delete", func() {
		It("tries to delete the blob from local blobsDir, and calls Delete on inner blobstore", func() {
			blobID := "smurf-25"

			err := fs.WriteFileString(path.Join(blobsDir, blobID), "smurf-content")
			Expect(err).To(BeNil())
			Expect(fs.FileExists(path.Join(blobsDir, blobID))).To(BeTrue())

			err = cascadingBlobstore.Delete(blobID)
			Expect(err).To(BeNil())

			Expect(fs.FileExists(path.Join(blobsDir, blobID))).To(BeFalse())

			Expect(innerBlobstore.DeleteBlobID).To(Equal("smurf-25"))
		})

		It("does not error if it can't find the file in local blobsDir, and calls Delete on inner blobstore", func() {
			blobID := "smurf-26"

			err := cascadingBlobstore.Delete(blobID)
			Expect(err).To(BeNil())

			Expect(innerBlobstore.DeleteBlobID).To(Equal("smurf-26"))
		})

		It("returns an error if the inner blobstore fails to delete", func() {
			innerBlobstore.DeleteErr = errors.New("error deleting")

			err := cascadingBlobstore.Delete("smurf-27")

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error deleting"))
		})
	})
})
