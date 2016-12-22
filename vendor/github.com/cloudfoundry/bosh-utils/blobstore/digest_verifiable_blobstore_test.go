package blobstore_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	fakeblob "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"

)

var _ = Describe("checksumVerifiableBlobstore", func() {
	const (
		fixturePath   = "test_assets/some.config"
		fixtureSHA1   = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	)

	var (
		innerBlobstore              *fakeblob.FakeBlobstore
		checksumVerifiableBlobstore boshblob.Blobstore
		correctDigest               boshcrypto.Digest
	)

	BeforeEach(func() {
		correctDigest = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, fixtureSHA1)
		innerBlobstore = &fakeblob.FakeBlobstore{}
		checksumVerifiableBlobstore = boshblob.NewDigestVerifiableBlobstore(innerBlobstore)
	})

	Describe("Get", func() {
		It("returns without an error if digest matches", func() {
			innerBlobstore.GetFileName = fixturePath

			fileName, err := checksumVerifiableBlobstore.Get("fake-blob-id", correctDigest)
			Expect(err).ToNot(HaveOccurred())

			Expect(innerBlobstore.GetBlobIDs).To(Equal([]string{"fake-blob-id"}))
			Expect(fileName).To(Equal(fixturePath))
		})

		It("returns error if digest does not match", func() {
			innerBlobstore.GetFileName = fixturePath

			incorrectDigest := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "some-incorrect-sha1")

			_, err := checksumVerifiableBlobstore.Get("fake-blob-id", incorrectDigest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Checking downloaded blob 'fake-blob-id'"))
		})

		It("returns error if inner blobstore getting fails", func() {
			innerBlobstore.GetError = errors.New("fake-get-error")

			_, err := checksumVerifiableBlobstore.Get("fake-blob-id", correctDigest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-get-error"))
		})
	})

	Describe("CleanUp", func() {
		It("delegates to inner blobstore to clean up", func() {
			err := checksumVerifiableBlobstore.CleanUp("/some/file")
			Expect(err).ToNot(HaveOccurred())

			Expect(innerBlobstore.CleanUpFileName).To(Equal("/some/file"))
		})

		It("returns error if inner blobstore cleaning up fails", func() {
			innerBlobstore.CleanUpErr = errors.New("fake-clean-up-error")

			err := checksumVerifiableBlobstore.CleanUp("/some/file")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-clean-up-error"))
		})
	})

	Describe("Delete", func() {
		It("delegates to inner blobstore to clean up", func() {
			err := checksumVerifiableBlobstore.Delete("some-blob")
			Expect(err).ToNot(HaveOccurred())

			Expect(innerBlobstore.DeleteBlobID).To(Equal("some-blob"))
		})

		It("returns error if inner blobstore cleaning up fails", func() {
			innerBlobstore.DeleteErr = errors.New("fake-clean-up-error")

			err := checksumVerifiableBlobstore.Delete("/some/file")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-clean-up-error"))
		})
	})

	Describe("Create", func() {
		It("delegates to inner blobstore to create blob and returns sha1 of returned blob", func() {
			innerBlobstore.CreateBlobID = "fake-blob-id"

			blobID, err := checksumVerifiableBlobstore.Create(fixturePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(blobID).To(Equal("fake-blob-id"))

			Expect(innerBlobstore.CreateFileNames[0]).To(Equal(fixturePath))
		})

		It("returns error if inner blobstore blob creation fails", func() {
			innerBlobstore.CreateErr = errors.New("fake-create-error")

			_, err := checksumVerifiableBlobstore.Create(fixturePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-create-error"))
		})
	})

	Describe("Validate", func() {
		It("delegates to inner blobstore to validate", func() {
			err := checksumVerifiableBlobstore.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns error if inner blobstore validation fails", func() {
			innerBlobstore.ValidateError = bosherr.Error("fake-validate-error")

			err := checksumVerifiableBlobstore.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-validate-error"))
		})
	})
})
