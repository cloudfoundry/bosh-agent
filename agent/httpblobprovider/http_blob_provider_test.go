package httpblobprovider_test

import (
	"fmt"
	"net/http"
	"os"

	. "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	"github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("HTTPBlobImpl", func() {
	var (
		fakeFileSystem *fakesys.FakeFileSystem
		server         *ghttp.Server
		tempFile       system.File
		blobProvider   *HTTPBlobImpl
	)

	BeforeEach(func() {
		fakeFileSystem = fakesys.NewFakeFileSystem()
		server = ghttp.NewServer()

		blobProvider = NewHTTPBlobImpl(fakeFileSystem, server.HTTPTestServer.Client())
	})

	Describe("Get", func() {
		var (
			// sha sums for "abc", the contents of our file
			sha1        = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "a9993e364706816aba3e25717850c26c9cd0d89d")
			sha512      = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA512, "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f")
			multiDigest = boshcrypto.MustNewMultipleDigest(sha1, sha512)
		)

		BeforeEach(func() {
			var err error

			tempFile, err = fakeFileSystem.OpenFile("fake-file", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
			Expect(err).ToNot(HaveOccurred())

			fakeFileSystem.ReturnTempFile = tempFile
		})

		AfterEach(func() {
			Expect(fakeFileSystem.RemoveAll(tempFile.Name())).To(Succeed())
		})

		It("downloads the file and returns the contents", func() {
			server.RouteToHandler("GET", "/success-get-signed-url",
				ghttp.CombineHandlers(
					ghttp.RespondWith(http.StatusOK, "abc"),
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Header.Get("key")).To(Equal("value"))
					}),
				),
			)

			filepath, err := blobProvider.Get(fmt.Sprintf("%s/success-get-signed-url", server.URL()), multiDigest, map[string]string{"key": "value"})
			Expect(err).NotTo(HaveOccurred())

			content, err := fakeFileSystem.ReadFile(filepath)
			Expect(err).NotTo(HaveOccurred())
			Expect(content).To(Equal([]byte("abc")))
		})

		It("does something when the server responds with a bad status code", func() {
			server.RouteToHandler("GET", "/bad-get-signed-url",
				ghttp.CombineHandlers(
					ghttp.RespondWith(http.StatusBadRequest, "fake-bad-contents"),
				),
			)

			_, err := blobProvider.Get(fmt.Sprintf("%s/bad-get-signed-url", server.URL()), multiDigest, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).ToNot(ContainSubstring(fmt.Sprintf("%s/bad-get-signed-url", server.URL())))
		})

		It("does something when the server responds with an error", func() {
			disconnectingRequestHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				Expect(err).NotTo(HaveOccurred())

				conn.Close()
			})

			server.RouteToHandler("GET", "/get-disconnecting-handler", disconnectingRequestHandler)

			_, err := blobProvider.Get(fmt.Sprintf("%s/get-disconnecting-handler", server.URL()), multiDigest, nil)
			Expect(err).To(HaveOccurred())
		})

		It("errors when content does not match provided digest", func() {
			server.RouteToHandler("GET", "/success-get-signed-url",
				ghttp.CombineHandlers(
					ghttp.RespondWith(http.StatusOK, "abc"),
				),
			)

			badsha1 := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "bad-a9993e364706816aba3e25717850c26c9cd0d89d")
			badsha512 := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA512, "bad-ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f")
			badMultiDigest := boshcrypto.MustNewMultipleDigest(badsha1, badsha512)

			_, err := blobProvider.Get(fmt.Sprintf("%s/success-get-signed-url", server.URL()), badMultiDigest, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Upload", func() {
		testUpload := func(filepath, signedURL string) (boshcrypto.MultipleDigest, error) {
			err := fakeFileSystem.WriteFileString(filepath, "abc")
			Expect(err).NotTo(HaveOccurred())

			actualDigests, err := blobProvider.Upload(signedURL, filepath, map[string]string{"key": "value"})
			return actualDigests, err
		}

		It("calculates the digest and uploads file", func() {
			server.RouteToHandler("PUT", "/success-signed-url",
				ghttp.CombineHandlers(
					ghttp.VerifyBody([]byte("abc")),
					ghttp.RespondWith(http.StatusCreated, ``),
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Header.Get("key")).To(Equal("value"))
					}),
				),
			)

			digest, err := testUpload("/some/path.tgz", fmt.Sprintf("%s/success-signed-url", server.URL()))
			Expect(err).NotTo(HaveOccurred())

			// sha sums for "abc", the contents of our file
			sha1 := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "a9993e364706816aba3e25717850c26c9cd0d89d")
			sha512 := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA512, "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f")
			Expect(digest.DigestFor(boshcrypto.DigestAlgorithmSHA1)).To(Equal(sha1))
			Expect(digest.DigestFor(boshcrypto.DigestAlgorithmSHA512)).To(Equal(sha512))
		})

		It("does something when the server responds with a bad status code", func() {
			server.RouteToHandler("PUT", "/bad-status-code",
				ghttp.CombineHandlers(
					ghttp.VerifyBody([]byte("abc")),
					ghttp.RespondWith(http.StatusBadRequest, ``),
				),
			)

			_, err := testUpload("/some/path.tgz", fmt.Sprintf("%s/bad-status-code", server.URL()))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).ToNot(ContainSubstring(fmt.Sprintf("%s/bad-status-code", server.URL())))
		})

		It("does something when the server responds with an error", func() {
			disconnectingRequestHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				Expect(err).NotTo(HaveOccurred())

				conn.Close()
			})

			server.RouteToHandler("PUT", "/disconnecting-handler", disconnectingRequestHandler)
			_, err := testUpload("/some/path.tgz", fmt.Sprintf("%s/disconnecting-handler", server.URL()))
			Expect(err).To(HaveOccurred())
		})
	})
})
