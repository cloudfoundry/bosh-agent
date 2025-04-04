package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/crypto"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	"github.com/cloudfoundry/bosh-agent/v2/agent/blobstore/blobstorefakes"
)

var _ = Describe("UploadBlobAction", func() {

	var (
		uploadBlobAction action.UploadBlobAction
		fakeBlobManager  *blobstorefakes.FakeBlobManagerInterface
	)

	BeforeEach(func() {
		fakeBlobManager = &blobstorefakes.FakeBlobManagerInterface{}
		uploadBlobAction = action.NewUploadBlobAction(fakeBlobManager)
	})

	AssertActionIsAsynchronous(uploadBlobAction)
	AssertActionIsNotPersistent(uploadBlobAction)
	AssertActionIsNotLoggable(uploadBlobAction)

	AssertActionIsNotResumable(uploadBlobAction)
	AssertActionIsNotCancelable(uploadBlobAction)

	Describe("Run", func() {
		Context("Payload Validation", func() {
			It("validates the payload using provided Checksum", func() {
				_, err := uploadBlobAction.Run(action.UploadBlobSpec{
					Payload:  "Y2xvdWRmb3VuZHJ5",
					Checksum: crypto.MustParseMultipleDigest("sha1:e578935e2f0613d68ba6a4fcc0d32754b52d334d"),
					BlobID:   "id",
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("validates the payload using provided sha256 Checksum", func() {
				_, err := uploadBlobAction.Run(action.UploadBlobSpec{
					Payload: "Y2xvdWRmb3VuZHJ5",
					Checksum: crypto.MustNewMultipleDigest(crypto.NewDigest(crypto.DigestAlgorithmSHA256,
						// echo -n 'cloudfoundry' | shasum -a 256
						"2ad453a5a20f9e110c40100c7f8eeb929070dd5abea32d7401ab74779b695e73")),
					BlobID: "id",
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not validate the payload when the Checksum is incorrect", func() {
				_, err := uploadBlobAction.Run(action.UploadBlobSpec{
					Payload:  "Y2xvdWRmb3VuZHJ5",
					Checksum: crypto.MustParseMultipleDigest("sha1:badChecksum"),
					BlobID:   "id",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Payload corrupted. Checksum mismatch. Expected 'badChecksum'"))
			})
		})

		It("should call the blob manager", func() {
			_, err := uploadBlobAction.Run(action.UploadBlobSpec{
				Payload:  "Y2xvdWRmb3VuZHJ5",
				Checksum: crypto.MustParseMultipleDigest("sha1:e578935e2f0613d68ba6a4fcc0d32754b52d334d"),
				BlobID:   "id",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeBlobManager.WriteCallCount()).To(Equal(1))
		})

		It("should return an error if the blob manager fails", func() {
			fakeBlobManager.WriteReturns(errors.New("blob write error"))
			_, err := uploadBlobAction.Run(action.UploadBlobSpec{
				Payload:  "Y2xvdWRmb3VuZHJ5",
				Checksum: crypto.MustParseMultipleDigest("sha1:e578935e2f0613d68ba6a4fcc0d32754b52d334d"),
				BlobID:   "id",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("blob write error"))
		})
	})
})
