package blobstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"github.com/cloudfoundry/bosh-agent/agent/blobstore"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	fakeblob "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("cascadingBlobstore", func() {
	var (
		innerBlobstore     *fakeblob.FakeBlobstore
		blobManager        *fakeblob.FakeBlobManagerInterface
		cascadingBlobstore boshblob.Blobstore
	)

	BeforeEach(func() {
		innerBlobstore = &fakeblob.FakeBlobstore{}
		blobManager = &fakeblob.FakeBlobManagerInterface{}
		logger := boshlog.NewLogger(boshlog.LevelNone)

		cascadingBlobstore = blobstore.NewCascadingBlobstore(innerBlobstore, blobManager, logger)
	})

	Describe("Get", func() {
		Describe("when blobManager returns the file path", func() {
			It("returns the path provided by the blobManager", func() {
				blobManager.GetPathReturns("/path/to-copy/of-blob", nil)

				filename, err := cascadingBlobstore.Get("blobID", "sha1")

				Expect(err).To(BeNil())
				Expect(filename).To(Equal("/path/to-copy/of-blob"))

				Expect(blobManager.GetPathCallCount()).To(Equal(1))
				Expect(blobManager.GetPathArgsForCall(0)).To(Equal("blobID"))

				Expect(innerBlobstore.GetBlobIDs).Should(BeEmpty())
			})
		})

		Describe("when blobManager returns an error", func() {
			It("delegates the action of getting the blob to inner blobstore", func() {
				blobID := "smurf-4"
				sha1 := "smurf-4-sha"

				blobManager.GetPathReturns("", errors.New("broken"))

				innerBlobstore.GetFileName = "/smurf-file/path"
				innerBlobstore.GetError = nil
				innerBlobstore.CreateBlobID = "createdBlobID"
				innerBlobstore.CreateFingerprint = "createdSha"

				filename, err := cascadingBlobstore.Get(blobID, sha1)

				Expect(blobManager.GetPathCallCount()).To(Equal(1))
				Expect(blobManager.GetPathArgsForCall(0)).To(Equal(blobID))

				Expect(err).To(BeNil())
				Expect(len(innerBlobstore.GetBlobIDs)).To(Equal(1))
				Expect(len(innerBlobstore.GetFingerprints)).To(Equal(1))

				Expect(innerBlobstore.GetBlobIDs[0]).To(Equal(blobID))
				Expect(innerBlobstore.GetFingerprints[0]).To(Equal(sha1))

				Expect(filename).To(Equal("/smurf-file/path"))
			})

			Describe("when inner blobstore returns an error", func() {
				It("returns that error to the caller", func() {
					blobID := "smurf-5"
					sha1 := "smurf-5-sha"

					blobManager.GetPathReturns("", errors.New("broken"))

					innerBlobstore.GetFileName = "/smurf-file/path"
					innerBlobstore.GetError = errors.New("inner blobstore GET is broken")

					_, err := cascadingBlobstore.Get(blobID, sha1)

					Expect(blobManager.GetPathCallCount()).To(Equal(1))
					Expect(blobManager.GetPathArgsForCall(0)).To(Equal(blobID))

					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("inner blobstore GET is broken"))
				})
			})
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
		It("deletes the blob from the blobManager, and calls Delete on inner blobstore", func() {
			blobID := "smurf-25"

			blobManager.DeleteReturns(nil)
			innerBlobstore.DeleteErr = nil

			err := cascadingBlobstore.Delete(blobID)

			Expect(err).To(BeNil())

			Expect(blobManager.DeleteCallCount()).To(Equal(1))
			Expect(blobManager.DeleteArgsForCall(0)).To(Equal(blobID))

			Expect(innerBlobstore.DeleteBlobID).To(Equal(blobID))
		})

		It("returns an error if blobManager returns an error when deleting", func() {
			blobID := "smurf-28"

			blobManager.DeleteReturns(errors.New("error deleting in blobManager"))
			innerBlobstore.DeleteErr = nil

			err := cascadingBlobstore.Delete(blobID)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error deleting in blobManager"))

			Expect(blobManager.DeleteCallCount()).To(Equal(1))
			Expect(blobManager.DeleteArgsForCall(0)).To(Equal(blobID))

			Expect(innerBlobstore.DeleteBlobID).To(Equal(""))
		})

		It("returns an error if inner blobStore returns an error when deleting", func() {
			blobID := "smurf-29"

			blobManager.DeleteReturns(nil)
			innerBlobstore.DeleteErr = errors.New("error deleting in innerBlobStore")

			err := cascadingBlobstore.Delete(blobID)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error deleting in innerBlobStore"))

			Expect(blobManager.DeleteCallCount()).To(Equal(1))
			Expect(blobManager.DeleteArgsForCall(0)).To(Equal(blobID))

			Expect(innerBlobstore.DeleteBlobID).To(Equal(blobID))
		})
	})
})
