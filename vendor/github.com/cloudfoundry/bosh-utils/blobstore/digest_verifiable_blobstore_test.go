package blobstore_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	fakeblob "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("checksumVerifiableBlobstore", func() {
	const (
		fixturePath   = "test_assets/some.config"
		fixtureSHA1   = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
		fixtureSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		fixtureSHA512 = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	)

	var (
		innerBlobstore              *fakeblob.FakeBlobstore
		checksumVerifiableBlobstore boshblob.Blobstore
		checksumProvider            boshcrypto.DigestProvider
		fixtureDigest               boshcrypto.Digest
	)

	BeforeEach(func() {
		fixtureDigest = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, fixtureSHA1)
		innerBlobstore = &fakeblob.FakeBlobstore{}
		checksumProvider = boshcrypto.NewDigestProvider(fakesys.NewFakeFileSystem())
		checksumVerifiableBlobstore = boshblob.NewDigestVerifiableBlobstore(innerBlobstore, checksumProvider)
	})

	Describe("Get", func() {
		It("returns without an error if sha1 matches", func() {
			innerBlobstore.GetFileName = fixturePath

			fileName, err := checksumVerifiableBlobstore.Get("fake-blob-id", fixtureDigest)
			Expect(err).ToNot(HaveOccurred())

			Expect(innerBlobstore.GetBlobIDs).To(Equal([]string{"fake-blob-id"}))
			Expect(fileName).To(Equal(fixturePath))
		})

		It("returns error if sha1 does not match", func() {
			innerBlobstore.GetFileName = fixturePath
			incorrectSha1 := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "some-incorrect-sha1")

			_, err := checksumVerifiableBlobstore.Get("fake-blob-id", incorrectSha1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Expected sha1 digest"))
		})

		It("returns error if inner blobstore getting fails", func() {
			innerBlobstore.GetError = errors.New("fake-get-error")

			_, err := checksumVerifiableBlobstore.Get("fake-blob-id", fixtureDigest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-get-error"))
		})

		It("skips sha1 verification and returns without an error if sha1 is empty", func() {
			innerBlobstore.GetFileName = fixturePath

			fileName, err := checksumVerifiableBlobstore.Get("fake-blob-id", nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(fileName).To(Equal(fixturePath))
		})

		Context("sha256", func() {
			BeforeEach(func() {
				fixtureDigest = boshcrypto.NewDigest("sha256", fixtureSHA256)
			})

			It("returns without an error if sha256 matches", func() {
				innerBlobstore.GetFileName = fixturePath

				fileName, err := checksumVerifiableBlobstore.Get("fake-blob-id", fixtureDigest)
				Expect(err).ToNot(HaveOccurred())

				Expect(innerBlobstore.GetBlobIDs).To(Equal([]string{"fake-blob-id"}))
				Expect(fileName).To(Equal(fixturePath))
			})
		})

		Context("sha512", func() {
			BeforeEach(func() {
				fixtureDigest = boshcrypto.NewDigest("sha512", fixtureSHA512)
			})

			It("returns without an error if sha256 matches", func() {
				innerBlobstore.GetFileName = fixturePath

				fileName, err := checksumVerifiableBlobstore.Get("fake-blob-id", fixtureDigest)
				Expect(err).ToNot(HaveOccurred())

				Expect(innerBlobstore.GetBlobIDs).To(Equal([]string{"fake-blob-id"}))
				Expect(fileName).To(Equal(fixturePath))
			})
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
