package crypto_test

import (
	. "github.com/cloudfoundry/bosh-utils/crypto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Algorithms", func() {
	Describe("sha1", func() {
		It("compares on algorithm strength", func() {
			var algorithm Algorithm
			Expect(DigestAlgorithmSHA1.Compare(algorithm)).To(Equal(1))
			Expect(DigestAlgorithmSHA1.Compare(DigestAlgorithmSHA1)).To(Equal(0))
			Expect(DigestAlgorithmSHA1.Compare(DigestAlgorithmSHA256)).To(Equal(-1))
			Expect(DigestAlgorithmSHA1.Compare(DigestAlgorithmSHA512)).To(Equal(-1))
		})
	})

	Describe("sha256", func() {
		It("compares on algorithm strength", func() {
			var algorithm Algorithm
			Expect(DigestAlgorithmSHA256.Compare(algorithm)).To(Equal(1))
			Expect(DigestAlgorithmSHA256.Compare(DigestAlgorithmSHA1)).To(Equal(1))
			Expect(DigestAlgorithmSHA256.Compare(DigestAlgorithmSHA256)).To(Equal(0))
			Expect(DigestAlgorithmSHA256.Compare(DigestAlgorithmSHA512)).To(Equal(-1))
		})

	})
	Describe("sha512", func() {
		It("compares on algorithm strength", func() {
			var algorithm Algorithm
			Expect(DigestAlgorithmSHA512.Compare(algorithm)).To(Equal(1))
			Expect(DigestAlgorithmSHA512.Compare(DigestAlgorithmSHA1)).To(Equal(1))
			Expect(DigestAlgorithmSHA512.Compare(DigestAlgorithmSHA256)).To(Equal(1))
			Expect(DigestAlgorithmSHA512.Compare(DigestAlgorithmSHA512)).To(Equal(0))
		})
	})
})
