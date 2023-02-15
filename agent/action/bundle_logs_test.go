package action_test

import (
	"errors"

	fakelogstarprovider "github.com/cloudfoundry/bosh-agent/agent/logstarprovider/logstarproviderfakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FetchLogsAction", func() {
	var (
		logsTarProvider *fakelogstarprovider.FakeLogsTarProvider
		fs              *fakesys.FakeFileSystem

		action BundleLogsAction
	)

	BeforeEach(func() {
		logsTarProvider = &fakelogstarprovider.FakeLogsTarProvider{}
		fs = fakesys.NewFakeFileSystem()

		action = NewBundleLogs(logsTarProvider, fs)
	})

	AssertActionIsLoggable(action)

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Describe("Run", func() {
		It("logs error if logstarprovider returns one", func() {
			logsTarProvider.GetReturns("", errors.New("uh-oh"))

			request := BundleLogsRequest{LogType: "other-logs", Filters: []string{}}
			_, err := action.Run(request)
			Expect(err).To(MatchError("uh-oh"))
		})

		It("invokes logstarprovider properly", func() {
			request := BundleLogsRequest{LogType: "job", Filters: []string{"foo", "bar"}}
			_, err := action.Run(request)
			Expect(err).ToNot(HaveOccurred())

			logType, filters := logsTarProvider.GetArgsForCall(0)
			Expect(logType).To(Equal("job"))
			Expect(filters).To(Equal([]string{"foo", "bar"}))

			Expect(logsTarProvider.CleanUpCallCount()).To(BeZero())
		})

		It("returns the expected logs tarball path", func() {
			logsTarProvider.GetReturns("/tmp/logsinhere.tgz", nil)

			request := BundleLogsRequest{LogType: "job", Filters: []string{"foo", "bar"}}
			logsPath, err := action.Run(request)
			Expect(err).ToNot(HaveOccurred())

			boshassert.MatchesJSONString(GinkgoT(), logsPath, `{"logs_tar_path":"/tmp/logsinhere.tgz"}`)
		})

		Context("chowning", func() {
			BeforeEach(func() {
				path := "/tmp/logsinhere.tgz"
				logsTarProvider.GetReturns(path, nil)
				fs.WriteFileString(path, "")
			})

			It("chowns the log tarball if provided a user", func() {
				request := BundleLogsRequest{OwningUser: "bosh_82398hcas", LogType: "job", Filters: []string{"foo", "bar"}}
				_, err := action.Run(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(fs.ChownCallCount).To(Equal(1))
			})

			It("does not chowns the log tarball if user not provided", func() {
				request := BundleLogsRequest{LogType: "job", Filters: []string{"foo", "bar"}}
				_, err := action.Run(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(fs.ChownCallCount).To(BeZero())
			})
		})
	})
})
