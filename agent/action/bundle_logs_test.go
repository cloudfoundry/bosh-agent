package action_test

import (
	"errors"

	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	fakelogstarprovider "github.com/cloudfoundry/bosh-agent/agent/logstarprovider/logstarproviderfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
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

		It("returns the expected logs tarball path and sha512", func() {
			logsTarProvider.GetReturns("/tmp/logsinhere.tgz", nil)

			request := BundleLogsRequest{LogType: "job", Filters: []string{"foo", "bar"}}
			logsPath, err := action.Run(request)
			Expect(err).ToNot(HaveOccurred())

			const emptyFileSHA512 string = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
			boshassert.MatchesJSONString(GinkgoT(), logsPath, `{"logs_tar_path":"/tmp/logsinhere.tgz","sha512":"sha512:`+emptyFileSHA512+`"}`)
		})

		Context("chowning", func() {
			BeforeEach(func() {
				path := "/tmp/logsinhere.tgz"
				logsTarProvider.GetReturns(path, nil)
				err := fs.WriteFileString(path, "")
				Expect(err).ToNot(HaveOccurred())
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
