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

		digest = NewMultipleDigest(digest1, digest2)
	})

	Describe("Verify", func() {
		var (
			digest Digest
		)

		Context("for a multi digest containing no digests", func() {
			BeforeEach(func() {
				digest = NewMultipleDigest()
			})

			It("does not Verify", func() {
				Expect(digest.Verify(strings.NewReader("desired content"))).To(HaveOccurred())
			})

		})

		Context("for a multi digest containing only SHA1 digest", func() {
			BeforeEach(func() {
				abcDigest, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("desired content"))
				Expect(err).ToNot(HaveOccurred())
				digest = NewMultipleDigest(abcDigest)
			})

			Context("when the checksum matches", func() {
				It("does not error", func() {
					Expect(digest.Verify(strings.NewReader("desired content"))).ToNot(HaveOccurred())
				})
			})

			Context("when the checksum does not match", func() {
				It("does errors", func() {
					Expect(digest.Verify(strings.NewReader("different content"))).To(HaveOccurred())
				})
			})
		})

		Context("for a multi digest containing many digest", func() {
			Context("when the strongest digest matches", func() {
				BeforeEach(func() {
					sha1DesiredContentDigest, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("weak digest content"))
					Expect(err).ToNot(HaveOccurred())
					sha256DesiredContentDigest, err := DigestAlgorithmSHA256.CreateDigest(strings.NewReader("weak digest content"))
					Expect(err).ToNot(HaveOccurred())
					sha512DesiredContentDigest, err := DigestAlgorithmSHA512.CreateDigest(strings.NewReader("strong desired content"))
					Expect(err).ToNot(HaveOccurred())

					digest = NewMultipleDigest(sha1DesiredContentDigest, sha256DesiredContentDigest, sha512DesiredContentDigest)
				})

				It("It favors the strongest digest and does not error", func() {
					Expect(digest.Verify(strings.NewReader("strong desired content"))).ToNot(HaveOccurred())
				})

				Context("when the checksum does not match", func() {
					It("does not error", func() {
						Expect(digest.Verify(strings.NewReader("weak digest content"))).To(HaveOccurred())
					})
				})
			})

			Context("when two of the digests are the same algorithm", func() {
				Context("when the two digests are equal", func() {
					BeforeEach(func() {
						sha1DesiredContentDigestA, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("digest content"))
						Expect(err).ToNot(HaveOccurred())
						sha1DesiredContentDigestB, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("digest content"))
						Expect(err).ToNot(HaveOccurred())

						digest = NewMultipleDigest(sha1DesiredContentDigestA, sha1DesiredContentDigestB)
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

						digest = NewMultipleDigest(sha1DesiredContentDigestA, sha1DesiredContentDigestB)
					})

					It("should not be verifiable", func() {
						err := digest.Verify(strings.NewReader("digest content A"))
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("multiple digests of the same algorithm with different checksums. Algorthim: 'sha1'"))
					})
				})
			})
		})

		Context("for a multi digest containing many digest", func() {
			BeforeEach(func() {
				sha1DesiredContentDigest, err := DigestAlgorithmSHA1.CreateDigest(strings.NewReader("weak digest content"))
				Expect(err).ToNot(HaveOccurred())
				sha256DesiredContentDigest, err := DigestAlgorithmSHA256.CreateDigest(strings.NewReader("weak digest content"))
				Expect(err).ToNot(HaveOccurred())
				sha512DesiredContentDigest, err := DigestAlgorithmSHA512.CreateDigest(strings.NewReader("strong desired content"))
				Expect(err).ToNot(HaveOccurred())

				digest = NewMultipleDigest(sha1DesiredContentDigest, sha256DesiredContentDigest, sha512DesiredContentDigest)
			})

			It("It favors the strongest digest and does not error", func() {
				Expect(digest.Verify(strings.NewReader("strong desired content"))).ToNot(HaveOccurred())
			})

			Context("when the strongest checksum does not match, but a weaker checksum does match", func() {
				It("errors", func() {
					Expect(digest.Verify(strings.NewReader("weak digest content"))).To(HaveOccurred())
				})
			})
		})
	})

	Describe("Unmarshalling JSON", func() {
		It("should produce valid JSON", func() {
			jsonString := "sha1:abcdefg;sha256:hijklmn"

			err := digest.UnmarshalJSON([]byte(jsonString))

			Expect(err).ToNot(HaveOccurred())
			Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA256))
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

		It("should throw an error if JSON does not contain any digests", func() {
			jsonString := ""

			err := digest.UnmarshalJSON([]byte(jsonString))

			Expect(err).To(HaveOccurred())
		})
	})
})
