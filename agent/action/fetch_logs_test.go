package action_test

import (
	"errors"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	fakelogstarprovider "github.com/cloudfoundry/bosh-agent/agent/logstarprovider/logstarproviderfakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchLogsAction", func() {
	var (
		blobstore       *fakeblobdelegator.FakeBlobstoreDelegator
		logsTarProvider *fakelogstarprovider.FakeLogsTarProvider

		action FetchLogsAction
	)

	BeforeEach(func() {
		blobstore = &fakeblobdelegator.FakeBlobstoreDelegator{}
		logsTarProvider = &fakelogstarprovider.FakeLogsTarProvider{}

		action = NewFetchLogs(logsTarProvider, blobstore)
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotPersistent(action)
	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Describe("Run", func() {
		It("logs error if logstarprovider returns one", func() {
			logsTarProvider.GetReturns("", errors.New("uh-oh"))
			_, err := action.Run("other-logs", []string{})
			Expect(err).To(MatchError("uh-oh"))
		})

		It("invokes logstarprovider properly", func() {
			_, err := action.Run("job", []string{"foo", "bar"})
			Expect(err).ToNot(HaveOccurred())

			logType, filters := logsTarProvider.GetArgsForCall(0)
			Expect(logType).To(Equal("job"))
			Expect(filters).To(Equal([]string{"foo", "bar"}))
		})

		It("returns the expected log blob", func() {
			multidigestSha := boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "sec_dep_sha1"))
			sha1 := multidigestSha.String()
			blobstore.WriteReturnsOnCall(0, "my-blob-id", multidigestSha, nil)

			logsBlob, err := action.Run("job", []string{"foo", "bar"})
			Expect(err).ToNot(HaveOccurred())

			boshassert.MatchesJSONString(GinkgoT(), logsBlob, `{"blobstore_id":"my-blob-id","sha1":"`+sha1+`"}`)
		})

		It("logs error if blobstore returns one", func() {
			blobstore.WriteReturns("", boshcrypto.MultipleDigest{}, errors.New("cloudy"))
			_, err := action.Run("agent", []string{})
			Expect(err).To(MatchError(ContainSubstring("Create file on blobstore")))
			Expect(err).To(MatchError(ContainSubstring("cloudy")))
		})

		It("cleans up compressed package only after uploading it to blobstore", func() {
			var beforeCallCount int
			blobstore.WriteStub = func(string, string, map[string]string) (string, boshcrypto.MultipleDigest, error) {
				beforeCallCount = logsTarProvider.CleanUpCallCount()

				return "", boshcrypto.MultipleDigest{}, nil
			}
			logsTarProvider.GetReturns("/tmp/logs.tar", nil)

			action.Run("job", []string{})

			Expect(beforeCallCount).To(BeZero())
			Expect(logsTarProvider.CleanUpCallCount()).To(Equal(1))
			Expect(logsTarProvider.CleanUpArgsForCall(0)).To(Equal("/tmp/logs.tar"))
		})
	})
})
