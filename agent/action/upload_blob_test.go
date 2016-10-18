package action_test

import (
	"errors"
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func init() {
	Describe("UploadBlobAction", func() {

		var (
			action          UploadBlobAction
			fakeBlobManager *FakeBlobManagerInterface
		)

		BeforeEach(func() {
			fakeBlobManager = &FakeBlobManagerInterface{}
			action = NewUploadBlobAction(fakeBlobManager)
		})

		AssertActionIsAsynchronous(action)
		AssertActionIsNotPersistent(action)
		AssertActionIsNotLoggable(action)

		AssertActionIsNotResumable(action)
		AssertActionIsNotCancelable(action)

		Describe("Run", func() {
			Context("Payload Validation", func() {
				It("validates the payload using provided SHA1", func() {
					_, err := action.Run("Y2xvdWRmb3VuZHJ5", "e578935e2f0613d68ba6a4fcc0d32754b52d334d", "id")
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not validate the payload when the SHA1 is incorrect", func() {
					_, err := action.Run("Y2xvdWRmb3VuZHJ5", "badsha1", "id")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Payload corrupted. SHA1 mismatch. Expected badsha1 but received e578935e2f0613d68ba6a4fcc0d32754b52d334d"))
				})
			})

			It("should call the blob manager", func() {
				_, err := action.Run("Y2xvdWRmb3VuZHJ5", "e578935e2f0613d68ba6a4fcc0d32754b52d334d", "id")
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeBlobManager.WriteCallCount()).To(Equal(1))
			})

			It("should return an error if the blob manager fails", func() {
				fakeBlobManager.WriteReturns(errors.New("blob write error"))
				_, err := action.Run("Y2xvdWRmb3VuZHJ5", "e578935e2f0613d68ba6a4fcc0d32754b52d334d", "id")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("blob write error"))
			})
		})
	})
}
