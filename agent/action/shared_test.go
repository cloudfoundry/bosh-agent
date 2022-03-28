package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
)

func AssertActionIsSynchronousForVersion(a action.Action, version action.ProtocolVersion) {
	It("is synchronous for version", func() {
		Expect(a.IsAsynchronous(version)).To(BeFalse())
	})
}

func AssertActionIsAsynchronousForVersion(a action.Action, version action.ProtocolVersion) {
	It("is synchronous for version", func() {
		Expect(a.IsAsynchronous(version)).To(BeTrue())
	})
}

func AssertActionIsAsynchronous(a action.Action) {
	It("is asynchronous", func() {
		Expect(a.IsAsynchronous(action.ProtocolVersion(1))).To(BeTrue())
	})
}

func AssertActionIsNotAsynchronous(a action.Action) {
	It("is not asynchronous", func() {
		Expect(a.IsAsynchronous(action.ProtocolVersion(1))).To(BeFalse())
	})
}

func AssertActionIsPersistent(a action.Action) {
	It("is persistent", func() {
		Expect(a.IsPersistent()).To(BeTrue())
	})
}

func AssertActionIsNotPersistent(a action.Action) {
	It("is not persistent", func() {
		Expect(a.IsPersistent()).To(BeFalse())
	})
}

func AssertActionIsLoggable(a action.Action) {
	It("is loggable", func() {
		Expect(a.IsLoggable()).To(BeTrue())
	})
}

func AssertActionIsNotLoggable(a action.Action) {
	It("is not loggable", func() {
		Expect(a.IsLoggable()).To(BeFalse())
	})
}

func AssertActionIsNotCancelable(a action.Action) {
	It("cannot be cancelled", func() {
		err := a.Cancel()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("not supported"))
	})
}

func AssertActionIsResumable(a action.Action) {
	It("can be resumed", func() {
		value, err := a.Resume()
		Expect(value).To(Equal("ok"))
		Expect(err).ToNot(HaveOccurred())
	})
}

func AssertActionIsNotResumable(a action.Action) {
	It("cannot be resumed", func() {
		_, err := a.Resume()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("not supported"))
	})
}
