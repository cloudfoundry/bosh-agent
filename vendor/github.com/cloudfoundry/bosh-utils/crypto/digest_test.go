package crypto_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-utils/crypto"
	"strings"
)


var _ = Describe("digest", func() {
	Describe("Verify", func() {
		Context("sha1", func() {
			var digest Digest

			BeforeEach(func() {
				digest = NewDigest(DigestAlgorithmSHA1, "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed")
			})

			It("returns nil when valid reader", func() {
				Expect(digest.Verify(strings.NewReader("hello world"))).To(BeNil())
			})

			It("returns error when invalid sum", func() {
				Expect(digest.Verify(strings.NewReader("omg"))).ToNot(BeNil())
			})
		})

		Context("sha256", func() {
			var digest Digest

			BeforeEach(func() {
				digest = NewDigest(DigestAlgorithmSHA256, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9")
			})

			It("returns nil when valid reader", func() {
				Expect(digest.Verify(strings.NewReader("hello world"))).To(BeNil())
			})

			It("returns error when invalid sum", func() {
				Expect(digest.Verify(strings.NewReader("omg"))).ToNot(BeNil())
			})
		})

		Context("sha512", func() {
			var digest Digest

			BeforeEach(func() {
				digest = NewDigest(DigestAlgorithmSHA512, "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f")
			})

			It("returns nil when valid reader", func() {
				Expect(digest.Verify(strings.NewReader("hello world"))).To(BeNil())
			})

			It("returns error when invalid sum", func() {
				Expect(digest.Verify(strings.NewReader("omg"))).ToNot(BeNil())
			})
		})
	})

	Describe("Digest", func() {
		Describe("#String", func() {
			Context("sha1", func() {
				It("excludes algorithm", func() {
					digest := NewDigest(DigestAlgorithmSHA1, "07e1306432667f916639d47481edc4f2ca456454")
					Expect(digest.String()).To(Equal("07e1306432667f916639d47481edc4f2ca456454"))
				})
			})

			Context("sha256", func() {
				It("includes algorithm", func() {
					digest := NewDigest(DigestAlgorithmSHA256, "b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23")
					Expect(digest.String()).To(Equal("sha256:b1e66f505465c28d705cf587b041a6506cfe749f7aa4159d8a3f45cc53f1fb23"))
				})
			})

			Context("sha512", func() {
				It("includes algorithm", func() {
					digest := NewDigest(DigestAlgorithmSHA512, "6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a")
					Expect(digest.String()).To(Equal("sha512:6f06a0c6c3827d827145b077cd8c8b7a15c75eb2bed809569296e6502ef0872c8e7ef91307a6994fcd2be235d3c41e09bfe1b6023df45697d88111df4349d64a"))
				})
			})
		})
	})
})
