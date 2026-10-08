package agentpassword_test

import (
	"encoding/base64"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agentpassword"
)

var _ = Describe("Agentpassword", func() {
	Describe("Feature constant", func() {
		It("exposes the expected feature identifier", func() {
			Expect(agentpassword.Feature).To(Equal("http-password-hmac-sha256"))
		})
	})

	Describe("IsVerifier", func() {
		It("returns true for verifiers and malformed verifiers with the prefix", func() {
			Expect(agentpassword.IsVerifier("bosh-hmac-sha256$bad")).To(BeTrue())
			Expect(agentpassword.IsVerifier("bosh-hmac-sha256$AAECAwQFBgcICQoLDA0ODw$EEqYVo55l7MGfPvwVPSRCq7RN6JXw-TBbKyW9De5Grw")).To(BeTrue())
		})

		It("returns false when the prefix is missing or incomplete", func() {
			Expect(agentpassword.IsVerifier("plain-password")).To(BeFalse())
			Expect(agentpassword.IsVerifier("")).To(BeFalse())
			Expect(agentpassword.IsVerifier("bosh-hmac-sha256")).To(BeFalse())
			Expect(agentpassword.IsVerifier("other-prefix$secret")).To(BeFalse())
		})
	})

	Describe("ParseVerifier and Matches", func() {
		It("matches a fixed known-answer vector guarding the format", func() {
			// salt: bytes 0..15 -> base64url "AAECAwQFBgcICQoLDA0ODw"
			// password: "correct horse battery staple"
			// HMAC-SHA256(key=salt, msg=password) -> base64url "EEqYVo55l7MGfPvwVPSRCq7RN6JXw-TBbKyW9De5Grw"
			const knownVector = "bosh-hmac-sha256$AAECAwQFBgcICQoLDA0ODw$EEqYVo55l7MGfPvwVPSRCq7RN6JXw-TBbKyW9De5Grw"
			v, err := agentpassword.ParseVerifier(knownVector)
			Expect(err).NotTo(HaveOccurred())
			Expect(v).NotTo(BeNil())
			Expect(v.Matches("correct horse battery staple")).To(BeTrue())
			Expect(v.Matches("wrong-password")).To(BeFalse())
		})

		It("rejects malformed verifiers", func() {
			validSalt := base64.RawURLEncoding.EncodeToString(make([]byte, 16))
			validKey := base64.RawURLEncoding.EncodeToString(make([]byte, 32))

			// Missing prefix
			for _, missingPrefix := range []string{
				"",
				"invalid-prefix$" + validSalt + "$" + validKey,
				"bosh-hmac-sha256",
			} {
				v, err := agentpassword.ParseVerifier(missingPrefix)
				Expect(err).To(HaveOccurred(), "expected error for missing prefix: %s", missingPrefix)
				Expect(v).To(BeNil())
			}

			// Bad part count
			for _, badParts := range []string{
				"bosh-hmac-sha256$",
				"bosh-hmac-sha256$" + validSalt,
				"bosh-hmac-sha256$" + validSalt + "$" + validKey + "$extra",
				"bosh-hmac-sha256$" + validSalt + "$" + validKey + "$extra$more",
			} {
				v, err := agentpassword.ParseVerifier(badParts)
				Expect(err).To(HaveOccurred(), "expected error for bad part count: %s", badParts)
				Expect(v).To(BeNil())
			}

			// Bad base64
			for _, badB64 := range []string{
				"bosh-hmac-sha256$invalid!salt$" + validKey,
				"bosh-hmac-sha256$" + validSalt + "$invalid!key",
			} {
				v, err := agentpassword.ParseVerifier(badB64)
				Expect(err).To(HaveOccurred(), "expected error for bad base64: %s", badB64)
				Expect(v).To(BeNil())
			}

			// Std base64 alphabet (+ and /) rejected by RawURLEncoding.Strict()
			for _, stdAlphabet := range []string{
				// PR #487 vector used std base64 '+' in the key
				"bosh-hmac-sha256$AAECAwQFBgcICQoLDA0ODw$EEqYVo55l7MGfPvwVPSRCq7RN6JXw+TBbKyW9De5Grw",
				// Salt containing '/'
				"bosh-hmac-sha256$AAECAwQFBgcICQoLDA0OD/$EEqYVo55l7MGfPvwVPSRCq7RN6JXw-TBbKyW9De5Grw",
			} {
				v, err := agentpassword.ParseVerifier(stdAlphabet)
				Expect(err).To(HaveOccurred(), "expected error for std base64 alphabet: %s", stdAlphabet)
				Expect(v).To(BeNil())
			}

			// Wrong salt length (must be 16 bytes)
			shortSalt := base64.RawURLEncoding.EncodeToString(make([]byte, 15))
			longSalt := base64.RawURLEncoding.EncodeToString(make([]byte, 17))
			for _, badSalt := range []string{
				"bosh-hmac-sha256$" + shortSalt + "$" + validKey,
				"bosh-hmac-sha256$" + longSalt + "$" + validKey,
			} {
				v, err := agentpassword.ParseVerifier(badSalt)
				Expect(err).To(HaveOccurred(), "expected error for wrong salt length: %s", badSalt)
				Expect(v).To(BeNil())
			}

			// Wrong key length (must be 32 bytes)
			shortKey := base64.RawURLEncoding.EncodeToString(make([]byte, 31))
			longKey := base64.RawURLEncoding.EncodeToString(make([]byte, 33))
			for _, badKey := range []string{
				"bosh-hmac-sha256$" + validSalt + "$" + shortKey,
				"bosh-hmac-sha256$" + validSalt + "$" + longKey,
			} {
				v, err := agentpassword.ParseVerifier(badKey)
				Expect(err).To(HaveOccurred(), "expected error for wrong key length: %s", badKey)
				Expect(v).To(BeNil())
			}
		})
	})

	Describe("HashURL", func() {
		It("performs a complete HashURL -> ParseVerifier -> Matches round trip", func() {
			raw := "https://vcap:mypassword@127.0.0.1:6868/path?foo=bar#frag"
			hashed, err := agentpassword.HashURL(raw)
			Expect(err).NotTo(HaveOccurred())

			parsedURL, err := url.Parse(hashed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsedURL.Scheme).To(Equal("https"))
			Expect(parsedURL.Host).To(Equal("127.0.0.1:6868"))
			Expect(parsedURL.Path).To(Equal("/path"))
			Expect(parsedURL.User.Username()).To(Equal("vcap"))

			pw, ok := parsedURL.User.Password()
			Expect(ok).To(BeTrue())

			v, err := agentpassword.ParseVerifier(pw)
			Expect(err).NotTo(HaveOccurred())
			Expect(v.Matches("mypassword")).To(BeTrue())
			Expect(v.Matches("wrong-password")).To(BeFalse())
		})

		It("produces different verifiers on two HashURL calls on the same input due to fresh salts", func() {
			raw := "https://vcap:mypassword@127.0.0.1:6868"
			h1, err := agentpassword.HashURL(raw)
			Expect(err).NotTo(HaveOccurred())
			h2, err := agentpassword.HashURL(raw)
			Expect(err).NotTo(HaveOccurred())

			Expect(h1).NotTo(Equal(h2))
		})

		It("ensures the password component contains only [A-Za-z0-9_$-] and appears unescaped in the returned string", func() {
			raw := "https://vcap:plain-password-with-chars@127.0.0.1:6868"
			hashed, err := agentpassword.HashURL(raw)
			Expect(err).NotTo(HaveOccurred())

			parsedURL, err := url.Parse(hashed)
			Expect(err).NotTo(HaveOccurred())
			pw, ok := parsedURL.User.Password()
			Expect(ok).To(BeTrue())

			Expect(pw).To(MatchRegexp(`^[A-Za-z0-9_$-]+$`))
			// Verifier appears unescaped in the returned string
			Expect(hashed).To(ContainSubstring(":" + pw + "@"))
		})

		It("passes non-https URLs through unchanged with nil error", func() {
			for _, nonHTTPS := range []string{
				"http://vcap:secret@127.0.0.1:6868",
				"nats://user:pass@127.0.0.1:4222",
				"custom://user:pass@127.0.0.1:8080/path",
				"",
			} {
				result, err := agentpassword.HashURL(nonHTTPS)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(nonHTTPS))
			}
		})

		It("returns an error and does not echo the password for every HashURL error case", func() {
			const secret = "very-secret-password-12345"

			testCases := []struct {
				name string
				raw  string
			}{
				{
					name: "unparseable URL with control character",
					raw:  "https://vcap:" + secret + "@127.0.0.1:6868\n/invalid",
				},
				{
					name: "missing userinfo",
					raw:  "https://127.0.0.1:6868",
				},
				{
					name: "absent password",
					raw:  "https://vcap@127.0.0.1:6868",
				},
				{
					name: "empty password",
					raw:  "https://vcap:@127.0.0.1:6868",
				},
				{
					name: "password already has verifier prefix",
					raw:  "https://vcap:bosh-hmac-sha256$" + secret + "@127.0.0.1:6868",
				},
			}

			for _, tc := range testCases {
				By(tc.name)
				hashed, err := agentpassword.HashURL(tc.raw)
				Expect(err).To(HaveOccurred())
				Expect(hashed).To(BeEmpty())
				Expect(err.Error()).NotTo(ContainSubstring(secret))
				Expect(err.Error()).NotTo(ContainSubstring("bosh-hmac-sha256$"))
			}
		})
	})
})
