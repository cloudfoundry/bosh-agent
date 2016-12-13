package crypto_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("MultipleDigest", func() {
	var (
		expectedDigest MultipleDigestImpl
		digest1        Digest
		digest2        Digest
	)

	BeforeEach(func() {
		digest1 = NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")
		digest2 = NewDigest(DigestAlgorithmSHA256, "07e1306432667f916639d47481edc4f2ca456454")

		expectedDigest = NewMultipleDigest(digest1, digest2)
	})

	Describe("Verify", func() {
		It("should select the highest algo and verify that digest", func() {
			actualDigest := NewDigest(DigestAlgorithmSHA256, "07e1306432667f916639d47481edc4f2ca456454")

			err := Verify(expectedDigest, actualDigest)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should work with only a single digest", func() {
			expectedDigest := NewMultipleDigest(digest1)
			actualDigest := NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")

			err := Verify(expectedDigest, actualDigest)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should throw an error if the digest does not match", func() {
			expectedDigest := NewMultipleDigest(digest1, digest2)
			actualDigest := NewDigest(DigestAlgorithmSHA256, "b1e66f505465c28d705cf587b041a6506cfe749f")

			err := Verify(expectedDigest, actualDigest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Expected sha256 digest \"07e1306432667f916639d47481edc4f2ca456454\" but received \"b1e66f505465c28d705cf587b041a6506cfe749f\""))
		})

		It("should throw an error if the algorithms do not match", func() {
			expectedDigest := NewMultipleDigest(digest1, digest2)
			actualDigest := NewDigest(DigestAlgorithmSHA512, "07e1306432667f916639d47481edc4f2ca456454")

			err := Verify(expectedDigest, actualDigest)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("No digest found that matches sha512"))
		})
	})

	Describe("Digests", func() {
		It("should return all the digests", func() {
			digest1 := NewDigest(DigestAlgorithmSHA512, "07e1306432667f916639d47481edc4f2ca456454")
			digest2 := NewDigest(DigestAlgorithmSHA512, "07e1306432667f916639d47481edc4f2ca456454")
			digest3 := NewDigest(DigestAlgorithmSHA512, "07e1306432667f916639d47481edc4f2ca456454")
			digests := []Digest{digest1, digest2, digest3}

			multiDigest := NewMultipleDigest(digest1, digest2, digest3)
			Expect(multiDigest.Digests()).To(Equal(digests))
		})

		It("should return an empty array if there are no digests", func() {
			multiDigest := NewMultipleDigest()
			Expect(multiDigest.Digests()).To(BeNil())
		})
	})

	Describe("Unmarshalling JSON", func() {
		It("should produce valid JSON", func() {
			jsonString := "sha1:abcdefg;sha256:hijklmn"

			err := expectedDigest.UnmarshalJSON([]byte(jsonString))

			Expect(err).ToNot(HaveOccurred())
			Expect(expectedDigest.Digests()).To(HaveLen(2))
		})

		Context("when given string has extra double quotes", func() {
			It("should unmarshal the string correctly, stripping the quotes", func() {
				jsonString := `"abcdefg"`

				err := expectedDigest.UnmarshalJSON([]byte(jsonString))

				Expect(err).ToNot(HaveOccurred())
				Expect(expectedDigest.Digests()).To(HaveLen(1))
				Expect(expectedDigest.Digests()[0].Digest()).To(Equal("abcdefg"))
			})
		})

		It("should throw an error if JSON does not contain a valid algorithm", func() {
			jsonString := "sha33:abcdefg;sha34:hijklmn"

			err := expectedDigest.UnmarshalJSON([]byte(jsonString))

			Expect(err).To(HaveOccurred())
		})
	})
})
