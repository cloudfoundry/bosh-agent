package agentclient_test

import (
	"crypto/x509"
	"errors"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/agentclient"
	fakeagentclient "github.com/cloudfoundry/bosh-agent/v2/agentclient/fakes"
)

var _ = Describe("PingRetryable", func() {
	Describe("Attempt", func() {
		var (
			fakeAgentClient *fakeagentclient.FakeAgentClient
			pingRetryable   boshretry.Retryable
		)

		BeforeEach(func() {
			fakeAgentClient = &fakeagentclient.FakeAgentClient{}
			pingRetryable = NewPingRetryable(fakeAgentClient)
		})

		It("tells the agent client to ping", func() {
			shouldRetry, err := pingRetryable.Attempt()
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRetry).To(BeFalse())
			Expect(fakeAgentClient.PingCallCount()).To(Equal(1))
		})

		Context("when pinging fails", func() {
			BeforeEach(func() {
				fakeAgentClient.PingReturns("", errors.New("fake-agent-client-ping-error"))
			})

			It("returns an error", func() {
				shouldRetry, err := pingRetryable.Attempt()
				Expect(err).To(HaveOccurred())
				Expect(shouldRetry).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("fake-agent-client-ping-error"))
			})
		})

		Context("when failing with a certificate error", func() {
			BeforeEach(func() {
				fakeAgentClient.PingReturns("", errors.New("some error with x509: stuff"))
			})

			It("returns stops retrying and returns the error", func() {
				shouldRetry, err := pingRetryable.Attempt()
				Expect(err).To(HaveOccurred())
				Expect(shouldRetry).To(BeFalse())
				Expect(err.Error()).To(ContainSubstring("some error with x509: stuff"))
			})

			Context("when the certificate error is wrapped", func() {
				BeforeEach(func() {
					certError := x509.CertificateInvalidError{}
					wrappedError := bosherr.WrapError(certError, "nope")
					doubleWrappedError := bosherr.WrapError(wrappedError, "nope nope")
					fakeAgentClient.PingReturns("", doubleWrappedError)
				})

				It("stops retrying and returns the error", func() {
					shouldRetry, err := pingRetryable.Attempt()
					Expect(err).To(HaveOccurred())
					Expect(shouldRetry).To(BeFalse())
					Expect(err.Error()).To(ContainSubstring("x509: certificate is not authorized to sign other certificates"))
				})
			})
		})
	})
})
