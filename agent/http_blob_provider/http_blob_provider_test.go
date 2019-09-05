package httpblobprovider_test

import (
	"fmt"
	"net/http"

	. "github.com/cloudfoundry/bosh-agent/agent/http_blob_provider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("HTTPBlobImpl", func() {
	var (
		fakeFileSystem *fakesys.FakeFileSystem
		server         *ghttp.Server
	)

	BeforeEach(func() {
		fakeFileSystem = fakesys.NewFakeFileSystem()
		server = ghttp.NewServer()
	})

	Describe("Upload", func() {
		testUpload := func(filepath, signedURL string) (boshcrypto.MultipleDigest, error) {
			err := fakeFileSystem.WriteFileString(filepath, "abc")
			Expect(err).NotTo(HaveOccurred())

			blobProvider := NewHTTPBlobImpl(fakeFileSystem).WithDefaultAlgorithms()

			actualDigests, err := blobProvider.Upload(signedURL, filepath)
			return actualDigests, err
		}

		It("calculates the digest and uploads file", func() {
			server.RouteToHandler("PUT", "/success-signed-url",
				ghttp.CombineHandlers(
					ghttp.VerifyBody([]byte("abc")),
					ghttp.VerifyHeaderKV("Content-Length", "3"),
					ghttp.RespondWith(http.StatusOK, ``),
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
					ghttp.VerifyHeaderKV("Content-Length", "3"),
					ghttp.RespondWith(http.StatusBadRequest, ``),
				),
			)

			_, err := testUpload("/some/path.tgz", fmt.Sprintf("%s/bad-status-code", server.URL()))
			Expect(err).To(HaveOccurred())
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
