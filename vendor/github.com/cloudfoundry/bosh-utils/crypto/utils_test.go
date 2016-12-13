package crypto_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	. "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("utils", func() {
	Describe("ParseDigestString", func() {
		Describe("sha1", func() {
			It("creates a digest", func() {
				digest, err := ParseDigestString("sha1:07e1306432667f916639d47481edc4f2ca456454")
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA1))
				Expect(digest.Digest()).To(Equal("07e1306432667f916639d47481edc4f2ca456454"))
			})
		})

		Describe("sha256", func() {
			It("creates a digest", func() {
				digest, err := ParseDigestString("sha256:b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23")
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA256))
				Expect(digest.Digest()).To(Equal("b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23"))
			})
		})

		Describe("sha512", func() {
			It("creates a digest", func() {
				digest, err := ParseDigestString("sha512:6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a")
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA512))
				Expect(digest.Digest()).To(Equal("6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a"))
			})
		})

		Describe("default", func() {
			It("creates a sha1 digest", func() {
				digest, err := ParseDigestString("07e1306432667f916639d47481edc4f2ca456454")
				Expect(err).ToNot(HaveOccurred())
				Expect(digest.Algorithm()).To(Equal(DigestAlgorithmSHA1))
				Expect(digest.Digest()).To(Equal("07e1306432667f916639d47481edc4f2ca456454"))
			})
		})

		Describe("unrecognized", func() {
			It("errors", func() {
				_, err := ParseDigestString("unrecognized:something")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Unrecognized digest algorithm: unrecognized. Supported algorithms: sha1, sha256, sha512"))
			})
		})

		Describe("empty string", func() {
			It("errors", func() {
				_, err := ParseDigestString("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Can not parse empty string."))
			})
		})
	})

	Describe("PreferredDigest", func() {
		Context("Given multiple digests", func() {
			It("returns the Digest with the strongest known algorithm", func() {
				multipleDigest, err := ParseMultipleDigestString("sha1:07e1306432667f916639d47481edc4f2ca456454;sha256:b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23;sha512:6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a")
				Expect(err).To(BeNil())

				strongestDigest, err := PreferredDigest(multipleDigest)
				Expect(err).To(BeNil())

				Expect(strongestDigest.Algorithm()).To(Equal(DigestAlgorithmSHA512))
				Expect(strongestDigest.Digest()).To(Equal("6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a"))
			})

			It("throws an error if there is no digest to choose from", func() {
				multipleDigest := NewMultipleDigest()

				strongestDigest, err := PreferredDigest(multipleDigest)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("No valid digests available"))

				Expect(strongestDigest.Digest()).To(BeEmpty())
			})
		})
	})

	Describe("ParseMultipleDigestString", func() {
		Context("single digest", func() {
			var multiDigest MultipleDigest

			BeforeEach(func() {
				var err error

				multiDigest, err = ParseMultipleDigestString("sha1:07e1306432667f916639d47481edc4f2ca456454")
				Expect(err).ToNot(HaveOccurred())
			})

			It("can verify the digest", func() {
				actual, err := ParseDigestString("sha1:07e1306432667f916639d47481edc4f2ca456454")
				Expect(err).ToNot(HaveOccurred())

				verifyErr := Verify(multiDigest, actual)
				Expect(verifyErr).ToNot(HaveOccurred())
			})

			It("can verify the digest even if no sha is prepended to the digest", func() {
				actualDigest := NewDigest("sha1", "07e1306432667f916639d47481edc4f2ca456454")
				parsedDigest, err := ParseMultipleDigestString("07e1306432667f916639d47481edc4f2ca456454")
				Expect(err).ToNot(HaveOccurred())

				Expect(Verify(parsedDigest, actualDigest)).To(BeNil())
			})
		})

		Context("multiple", func() {
			It("can verify the digest", func() {
				multipleDigest, err := ParseMultipleDigestString("sha1:07e1306432667f916639d47481edc4f2ca456454;sha256:b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23")

				Expect(err).To(BeNil())
				digest1 := NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")
				verifyDigest1Err := Verify(multipleDigest, digest1)
				Expect(verifyDigest1Err).ToNot(HaveOccurred())

				digest2 := NewDigest(DigestAlgorithmSHA256, "b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23")
				verifyDigest2Err := Verify(multipleDigest, digest2)
				Expect(verifyDigest2Err).ToNot(HaveOccurred())
			})

			It("parses list of recognizable digests", func() {
				multipleDigest, err := ParseMultipleDigestString("sha1:07e1306432667f916639d47481edc4f2ca456454;sha10:something")
				Expect(err).To(BeNil())

				digest1 := NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")
				Expect(Verify(multipleDigest, digest1)).NotTo(HaveOccurred())
			})

			It("returns an error if no digest is recongized", func() {
				_, err := ParseMultipleDigestString("sha10:something")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("No recognizable digest algorithm found. Supported algorithms: sha1, sha256, sha512"))
			})
		})
	})

	Describe("CreateHashFromAlgorithm", func() {
		data := []byte("the checksum of c1oudc0w is deterministic")

		Describe("sha1", func() {
			It("hashes", func() {
				hash, err := CreateHashFromAlgorithm("sha1")
				Expect(err).ToNot(HaveOccurred())

				hash.Write(data)
				Expect(fmt.Sprintf("%x", hash.Sum(nil))).To(Equal("07e1306432667f916639d47481edc4f2ca456454"))
			})
		})

		Describe("sha256", func() {
			It("hashes", func() {
				hash, err := CreateHashFromAlgorithm("sha256")
				Expect(err).ToNot(HaveOccurred())

				hash.Write(data)
				Expect(fmt.Sprintf("%x", hash.Sum(nil))).To(Equal("b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23"))
			})
		})

		Describe("sha512", func() {
			It("hashes", func() {
				hash, err := CreateHashFromAlgorithm("sha512")
				Expect(err).ToNot(HaveOccurred())

				hash.Write(data)
				Expect(fmt.Sprintf("%x", hash.Sum(nil))).To(Equal("6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a"))
			})
		})

		Describe("unrecognized", func() {
			It("errors", func() {
				_, err := CreateHashFromAlgorithm("unrecognized")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
