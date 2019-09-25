package blobstore_delegator_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/http_blob_provider/blobstore_delegator"
	fakeblobprovider "github.com/cloudfoundry/bosh-agent/agent/http_blob_provider/http_blob_providerfakes"
	fakeblobstore "github.com/cloudfoundry/bosh-utils/blobstore/fakes"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("BlobstoreDelegator", func() {
	var (
		blobstoreDelegator   blobstore_delegator.BlobstoreDelegator
		fakeHttpBlobProvider *fakeblobprovider.FakeHTTPBlobProvider
		fakeBlobManager      *fakeblobstore.FakeDigestBlobstore

		digest = boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "some-digest"))
	)

	BeforeEach(func() {
		fakeHttpBlobProvider = &fakeblobprovider.FakeHTTPBlobProvider{}
		fakeBlobManager = &fakeblobstore.FakeDigestBlobstore{}

		blobstoreDelegator = blobstore_delegator.NewBlobstoreDelegator(fakeHttpBlobProvider, fakeBlobManager)
	})

	Context("Get", func() {
		Context("when there is a signed URL provided", func() {
			It("reaches out to the HTTP blobstore", func() {
				downloadedFilePath := "/some/path/to/a/file"
				fakeHttpBlobProvider.GetReturns(downloadedFilePath, nil)
				getResponse, err := blobstoreDelegator.Get(digest, "some-signed-url", "")
				Expect(err).ToNot(HaveOccurred())
				Expect(getResponse).To(Equal(downloadedFilePath))

				Expect(fakeBlobManager.GetCallCount()).To(Equal(0))
				Expect(fakeHttpBlobProvider.GetCallCount()).To(Equal(1))

				signedURLArg, digestArg := fakeHttpBlobProvider.GetArgsForCall(0)
				Expect(signedURLArg).To(Equal("some-signed-url"))
				Expect(digestArg).To(Equal(digest))
			})

			It("errors when there is an error", func() {
				downloadedFilePath := "/some/path/to/a/file"
				fakeError := errors.New("some error")
				fakeHttpBlobProvider.GetReturns(downloadedFilePath, fakeError)

				_, err := blobstoreDelegator.Get(digest, "some-signed-url", "")
				Expect(err).To(MatchError(fakeError))

				Expect(fakeBlobManager.GetCallCount()).To(Equal(0))
				Expect(fakeHttpBlobProvider.GetCallCount()).To(Equal(1))
			})
		})

		Context("when there is no signed URL provided", func() {
			It("uses the local blobstore", func() {
				downloadedFilePath := "/some/path/to/a/file"
				fakeBlobManager.GetReturns(downloadedFilePath, nil)

				getResponse, err := blobstoreDelegator.Get(digest, "", "1234")
				Expect(err).ToNot(HaveOccurred())
				Expect(getResponse).To(Equal(downloadedFilePath))

				Expect(fakeBlobManager.GetCallCount()).To(Equal(1))
				Expect(fakeHttpBlobProvider.GetCallCount()).To(Equal(0))

				fetchedBlobID, digestArg := fakeBlobManager.GetArgsForCall(0)
				Expect(fetchedBlobID).To(Equal("1234"))
				Expect(digestArg).To(Equal(digest))
			})

			It("errors when there is an error", func() {
				downloadedFilePath := "/some/path/to/a/file"
				fakeError := errors.New("some error")
				fakeBlobManager.GetReturns(downloadedFilePath, fakeError)

				_, err := blobstoreDelegator.Get(digest, "", "1234")
				Expect(err).To(MatchError(fakeError))

				Expect(fakeBlobManager.GetCallCount()).To(Equal(1))
				Expect(fakeHttpBlobProvider.GetCallCount()).To(Equal(0))
			})
		})

		Context("when neither signedURL nor blobID are provided", func() {
			It("returns an error", func() {
				_, err := blobstoreDelegator.Get(digest, "", "")
				Expect(err).To(MatchError(errors.New("Both signedURL and blobID are blank which is invalid")))
			})
		})
	})

	Context("Write", func() {
		Context("when there is a signed URL provided", func() {
			It("reaches out to the HTTP blobstore", func() {
				filePath := "/some/path/to/a/file"
				fakeHttpBlobProvider.UploadReturns(digest, nil)

				_, digestResult, err := blobstoreDelegator.Write("some-signed-url", filePath)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBlobManager.CreateCallCount()).To(Equal(0))
				Expect(fakeHttpBlobProvider.UploadCallCount()).To(Equal(1))

				signedURLArg, filepathArg := fakeHttpBlobProvider.UploadArgsForCall(0)
				Expect(signedURLArg).To(Equal("some-signed-url"))
				Expect(filepathArg).To(Equal(filePath))
				Expect(digestResult).To(Equal(digest))
			})

			It("errors when there is an error", func() {
				filePath := "/some/path/to/a/file"
				fakeError := errors.New("some error")
				fakeHttpBlobProvider.UploadReturns(digest, fakeError)

				_, digestResult, err := blobstoreDelegator.Write("some-signed-url", filePath)
				Expect(err).To(MatchError(fakeError))
				Expect(fakeBlobManager.CreateCallCount()).To(Equal(0))
				Expect(fakeHttpBlobProvider.UploadCallCount()).To(Equal(1))

				signedURLArg, filepathArg := fakeHttpBlobProvider.UploadArgsForCall(0)
				Expect(signedURLArg).To(Equal("some-signed-url"))
				Expect(filepathArg).To(Equal(filePath))
				Expect(digestResult).To(Equal(digest))
			})
		})

		Context("when there is no signed URL provided", func() {
			It("uses the local blobstore", func() {
				filePath := "/some/path/to/a/file"
				fakeBlobManager.CreateReturns("123", digest, nil)

				blobID, digestResult, err := blobstoreDelegator.Write("", filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(blobID).To(Equal("123"))

				Expect(fakeBlobManager.CreateCallCount()).To(Equal(1))
				Expect(fakeHttpBlobProvider.UploadCallCount()).To(Equal(0))

				filenameArg := fakeBlobManager.CreateArgsForCall(0)
				Expect(filenameArg).To(Equal(filePath))
				Expect(digestResult).To(Equal(digest))
			})

			It("errors when there is an error", func() {
				fakeError := errors.New("some error")
				filePath := "/some/path/to/a/file"
				fakeBlobManager.CreateReturns("123", digest, fakeError)

				_, _, err := blobstoreDelegator.Write("", filePath)
				Expect(err).To(MatchError(fakeError))
				Expect(fakeBlobManager.CreateCallCount()).To(Equal(1))
				Expect(fakeHttpBlobProvider.UploadCallCount()).To(Equal(0))
			})
		})
	})

	Context("CleanUp", func() {
		Context("when there is a signed URL provided", func() {
			It("errors", func() {
				err := blobstoreDelegator.CleanUp("some-signed-url", "nothing")
				Expect(err).To(MatchError("CleanUp is not supported for signed URLs"))
			})
		})

		Context("when there is no signed URL provided", func() {
			It("Cleans up", func() {
				someFile := "/some/file"
				err := blobstoreDelegator.CleanUp("", someFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBlobManager.CleanUpCallCount()).To(Equal(1))
				Expect(fakeBlobManager.CleanUpArgsForCall(0)).To(Equal("/some/file"))
			})
		})
	})

	Context("Delete", func() {
		Context("when there is a signed URL provided", func() {
			It("errors", func() {
				err := blobstoreDelegator.Delete("some-signed-url", "nothing")
				Expect(err).To(MatchError("Delete is not supported for signed URLs"))
			})
		})

		Context("when there is no signed URL provided", func() {
			It("Deletes", func() {
				blobID := "123"
				err := blobstoreDelegator.Delete("", blobID)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBlobManager.DeleteCallCount()).To(Equal(1))
				Expect(fakeBlobManager.DeleteArgsForCall(0)).To(Equal("123"))
			})
		})
	})
})
