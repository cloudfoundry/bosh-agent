package action_test

import (
	"errors"

	fakelogstarprovider "github.com/cloudfoundry/bosh-agent/agent/logstarprovider/logstarproviderfakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchLogsAction", func() {
	var (
		logsTarProvider *fakelogstarprovider.FakeLogsTarProvider

		action BundleLogsAction
	)

	BeforeEach(func() {
		logsTarProvider = &fakelogstarprovider.FakeLogsTarProvider{}

		action = NewBundleLogs(logsTarProvider)
	})

	AssertActionIsLoggable(action)

	AssertActionIsNotAsynchronous(action)
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

			Expect(logsTarProvider.CleanUpCallCount()).To(BeZero())
		})

		It("returns the expected logs tar path", func() {
			logsTarProvider.GetReturns("/tmp/logsinhere.tgz", nil)

			logsPath, err := action.Run("job", []string{"foo", "bar"})
			Expect(err).ToNot(HaveOccurred())

			boshassert.MatchesJSONString(GinkgoT(), logsPath, `{"logs_tar_path":"/tmp/logsinhere.tgz"}`)
		})
	})
})
