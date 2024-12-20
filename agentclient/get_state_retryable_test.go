package agentclient_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/v2/agentclient"
	fakeagentclient "github.com/cloudfoundry/bosh-agent/v2/agentclient/fakes"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetStateRetryable", func() {
	Describe("Attempt", func() {
		var (
			fakeAgentClient   *fakeagentclient.FakeAgentClient
			getStateRetryable boshretry.Retryable
		)

		BeforeEach(func() {
			fakeAgentClient = &fakeagentclient.FakeAgentClient{}
			getStateRetryable = NewGetStateRetryable(fakeAgentClient)
		})

		Context("when get_state fails", func() {
			BeforeEach(func() {
				fakeAgentClient.GetStateReturns(AgentState{}, errors.New("fake-get-state-error"))
			})

			It("returns an error", func() {
				shouldRetry, err := getStateRetryable.Attempt()
				Expect(err).To(HaveOccurred())
				Expect(shouldRetry).To(BeFalse())
				Expect(err.Error()).To(ContainSubstring("fake-get-state-error"))
				Expect(fakeAgentClient.GetStateCallCount()).To(Equal(1))
			})
		})

		Context("when get_state returns state as pending", func() {
			BeforeEach(func() {
				fakeAgentClient.GetStateReturns(AgentState{JobState: "pending"}, nil)
			})

			It("returns retryable error", func() {
				shouldRetry, err := getStateRetryable.Attempt()
				Expect(err).To(HaveOccurred())
				Expect(shouldRetry).To(BeTrue())
				Expect(fakeAgentClient.GetStateCallCount()).To(Equal(1))
			})
		})

		Context("when get_state returns state as running", func() {
			BeforeEach(func() {
				fakeAgentClient.GetStateReturns(AgentState{JobState: "running"}, nil)
			})

			It("returns no error", func() {
				shouldRetry, err := getStateRetryable.Attempt()
				Expect(err).ToNot(HaveOccurred())
				Expect(shouldRetry).To(BeFalse())
				Expect(fakeAgentClient.GetStateCallCount()).To(Equal(1))
			})
		})
	})
})
