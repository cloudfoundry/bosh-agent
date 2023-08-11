package action_test

import (
	"errors"

	fakelogstarprovider "github.com/cloudfoundry/bosh-agent/agent/logstarprovider/logstarproviderfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"

	boshaction "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
)

var _ = Describe("FetchLogsWithSignedURLAction", func() {
	var (
		blobstore       *fakeblobdelegator.FakeBlobstoreDelegator
		logsTarProvider *fakelogstarprovider.FakeLogsTarProvider

		action boshaction.FetchLogsWithSignedURLAction
	)

	BeforeEach(func() {
		blobstore = &fakeblobdelegator.FakeBlobstoreDelegator{}
		logsTarProvider = &fakelogstarprovider.FakeLogsTarProvider{}

		action = boshaction.NewFetchLogsWithSignedURLAction(logsTarProvider, blobstore)
	})

	AssertActionIsAsynchronous(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotPersistent(action)
	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Describe("Run", func() {
		It("logs error if logstarprovider returns one", func() {
			logsTarProvider.GetReturns("", errors.New("uh-oh"))
			_, err := action.Run(boshaction.FetchLogsWithSignedURLRequest{
				SignedURL:        "foobar",
				LogType:          "other-logs",
				Filters:          []string{},
				BlobstoreHeaders: map[string]string{},
			})
			Expect(err).To(MatchError("uh-oh"))
		})

		It("invokes logstarprovider properly", func() {
			_, err := action.Run(boshaction.FetchLogsWithSignedURLRequest{
				SignedURL:        "foobar",
				LogType:          "job",
				Filters:          []string{"foo", "bar"},
				BlobstoreHeaders: map[string]string{},
			})
			Expect(err).ToNot(HaveOccurred())

			logType, filters := logsTarProvider.GetArgsForCall(0)
			Expect(logType).To(Equal("job"))
			Expect(filters).To(Equal([]string{"foo", "bar"}))
		})

		It("returns the expected log blob", func() {
			multidigestSha := boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "sec_dep_sha1"))
			sha1 := multidigestSha.String()
			blobstore.WriteReturnsOnCall(0, "my-blob-id", multidigestSha, nil)

			logsBlob, err := action.Run(boshaction.FetchLogsWithSignedURLRequest{
				SignedURL:        "foobar",
				LogType:          "job",
				Filters:          []string{"foo", "bar"},
				BlobstoreHeaders: map[string]string{},
			})
			Expect(err).ToNot(HaveOccurred())

			boshassert.MatchesJSONString(GinkgoT(), logsBlob, `{"sha1":"`+sha1+`"}`)
		})

		It("logs error if blobstore returns one", func() {
			blobstore.WriteReturns("", boshcrypto.MultipleDigest{}, errors.New("cloudy"))
			_, err := action.Run(boshaction.FetchLogsWithSignedURLRequest{
				SignedURL:        "foobar",
				LogType:          "agent",
				Filters:          []string{"foo", "bar"},
				BlobstoreHeaders: map[string]string{},
			})
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

			_, err := action.Run(boshaction.FetchLogsWithSignedURLRequest{
				SignedURL:        "foobar",
				LogType:          "job",
				Filters:          []string{"foo", "bar"},
				BlobstoreHeaders: map[string]string{},
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(beforeCallCount).To(BeZero())
			Expect(logsTarProvider.CleanUpCallCount()).To(Equal(1))
			Expect(logsTarProvider.CleanUpArgsForCall(0)).To(Equal("/tmp/logs.tar"))
		})
	})
})
