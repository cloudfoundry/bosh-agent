package crypto_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-utils/crypto"
	"strings"
)

var _ = Describe("MultipleDigest", func() {
	var (
		digest MultipleDigest
		digest1 Digest
		digest2 Digest
	)

	BeforeEach(func() {
		digest1 = NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")
		digest2 = NewDigest(DigestAlgorithmSHA256, "07e1306432667f916639d47481edc4f2ca456454")

		digest = MustNewMultipleDigest(digest1, digest2)
	})

	Describe("Verify", func() {
		var (
			digest Digest
		)

		Context("for a multi digest containing no digests", func() {
			BeforeEach(func() {
				digest = MultipleDigest{}
			})

			It("does not Verify", func() {
				Expect(digest.Verify(strings.NewReader("desired content"))).To(HaveOccurred())
			})
		})

		Context("for a multi digest containing only SHA1 digest", func() {
			BeforeEach(func() {
				abcDigest, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("desired content"))
				Expect(err).ToNot(HaveOccurred())
				digest = MustNewMultipleDigest(abcDigest)
			})

			It("does not error when the checksum matches", func() {
				Expect(digest.Verify(strings.NewReader("desired content"))).ToNot(HaveOccurred())
			})

			It("errors when the checksum does not match", func() {
				Expect(digest.Verify(strings.NewReader("different content"))).To(HaveOccurred())
			})
		})

		Context("for a multi digest containing many digests", func() {
			Context("when the strongest digest matches", func() {
				BeforeEach(func() {
					sha1DesiredContentDigest, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("weak digest content"))
					Expect(err).ToNot(HaveOccurred())
					sha256DesiredContentDigest, err := DigestAlgorithmSHA256.CreateDigest(strings.NewReader("weak digest content"))
					Expect(err).ToNot(HaveOccurred())
					sha512DesiredContentDigest, err := DigestAlgorithmSHA512.CreateDigest(strings.NewReader("strong desired content"))
					Expect(err).ToNot(HaveOccurred())

					digest = MustNewMultipleDigest(sha1DesiredContentDigest, sha256DesiredContentDigest, sha512DesiredContentDigest)
				})

				It("It uses the strongest digest and does not error", func() {
					Expect(digest.Verify(strings.NewReader("strong desired content"))).ToNot(HaveOccurred())
				})

				It("errors when the content does not match the strongest digest (even if it does match weaker digests)", func() {
					Expect(digest.Verify(strings.NewReader("weak digest content"))).To(HaveOccurred())
				})
			})

			Context("when two of the digests are the same algorithm", func() {
				Context("when the two digests are equal", func() {
					BeforeEach(func() {
						sha1DesiredContentDigestA, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("digest content"))
						Expect(err).ToNot(HaveOccurred())
						sha1DesiredContentDigestB, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("digest content"))
						Expect(err).ToNot(HaveOccurred())

						digest = MustNewMultipleDigest(sha1DesiredContentDigestA, sha1DesiredContentDigestB)
					})

					It("should be verifiable", func() {
						Expect(digest.Verify(strings.NewReader("digest content"))).ToNot(HaveOccurred())
					})
				})

				Context("when the two digests are not equal", func() {
					BeforeEach(func() {
						sha1DesiredContentDigestA, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("digest content A"))
						Expect(err).ToNot(HaveOccurred())
						sha1DesiredContentDigestB, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("digest content B"))
						Expect(err).ToNot(HaveOccurred())

						digest = MustNewMultipleDigest(sha1DesiredContentDigestA, sha1DesiredContentDigestB)
					})

					It("should not be verifiable", func() {
						err := digest.Verify(strings.NewReader("digest content A"))
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("multiple digests of the same algorithm with different checksums. Algorthim: 'sha1'"))
					})
				})
			})
		})
	})

	Describe("Unmarshalling JSON", func() {
		It("should parse from valid JSON", func() {
			jsonString := "sha1:abcdefg;sha256:1bf4b70c96b9d4e8f473ac6b7e6b5b965ab3497287a86eb2ed1b263287c78038"

			err := digest.UnmarshalJSON([]byte(jsonString))

			Expect(err).ToNot(HaveOccurred())
			Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA256))
			Expect(digest.Verify(strings.NewReader("content to be verified"))).ToNot(HaveOccurred())
		})

		Context("when given string has extra double quotes", func() {
			It("should unmarshal the string correctly, stripping the quotes", func() {
				jsonString := `"abcdefg"`

				err := digest.UnmarshalJSON([]byte(jsonString))

				Expect(err).ToNot(HaveOccurred())
				Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA1))
				Expect(digest.String()).To(Equal("abcdefg"))
			})
		})

		It("should throw an error if JSON does not contain a valid algorithm", func() {
			jsonString := "sha33:abcdefg;sha34:hijklmn"

			err := digest.UnmarshalJSON([]byte(jsonString))

			Expect(err).To(HaveOccurred())
		})

		Context("when the json contains a valid digest and an unknown digest", func() {
			It("should not error when unmarsahlling", func() {
				jsonString := "sha1024:abcdefg;sha256:1bf4b70c96b9d4e8f473ac6b7e6b5b965ab3497287a86eb2ed1b263287c78038"

				err := digest.UnmarshalJSON([]byte(jsonString))

				Expect(err).ToNot(HaveOccurred())
				Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA256))
				Expect(digest.Verify(strings.NewReader("content to be verified"))).ToNot(HaveOccurred())
			})
		})

		It("should throw an error if JSON does not contain any digests", func() {
			Expect(digest.UnmarshalJSON([]byte(""))).To(HaveOccurred())
		})
	})
})
