package drain_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	. "github.com/cloudfoundry/bosh-agent/agent/drain"
)

var _ = Describe("NewShutdownParams", func() {
	oldSpec := boshas.V1ApplySpec{PersistentDisk: 200}
	newSpec := boshas.V1ApplySpec{PersistentDisk: 301}

	Describe("JobState", func() {
		It("returns JSON serialized current spec that only includes persistent disk", func() {
			state, err := NewShutdownParams(oldSpec, &newSpec).JobState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":200}`))
		})
	})

	Describe("JobNextState", func() {
		It("returns JSON serialized future spec that only includes persistent disk", func() {
			state, err := NewShutdownParams(oldSpec, &newSpec).JobNextState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":301}`))
		})

		It("returns empty string if next state is not available", func() {
			state, err := NewShutdownParams(oldSpec, nil).JobNextState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(""))
		})
	})
})

var _ = Describe("NewStatusParams", func() {
	oldSpec := boshas.V1ApplySpec{PersistentDisk: 200}
	newSpec := boshas.V1ApplySpec{PersistentDisk: 301}

	Describe("JobState", func() {
		It("returns JSON serialized current spec that only includes persistent disk", func() {
			state, err := NewStatusParams(oldSpec, &newSpec).JobState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":200}`))
		})
	})

	Describe("JobNextState", func() {
		It("returns JSON serialized future spec that only includes persistent disk", func() {
			state, err := NewStatusParams(oldSpec, &newSpec).JobNextState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":301}`))
		})

		It("returns empty string if next state is not available", func() {
			state, err := NewStatusParams(oldSpec, nil).JobNextState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(""))
		})
	})
})
