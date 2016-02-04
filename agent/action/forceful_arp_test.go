package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
)

func init() {
	Describe("Forceful ARP", func() {
		It("is synchronous", func() {
			action := NewForcefulARP()
			Expect(action.IsAsynchronous()).To(BeFalse())
		})

		It("is not persistent", func() {
			action := NewForcefulARP()
			Expect(action.IsPersistent()).To(BeFalse())
		})
	})
}
