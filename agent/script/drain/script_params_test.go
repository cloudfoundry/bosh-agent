package drain_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/crypto"

	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	. "github.com/cloudfoundry/bosh-agent/v2/agent/script/drain"
)

var _ = Describe("NewShutdownParams", func() {
	var (
		oldSpec, newSpec boshas.V1ApplySpec
	)

	BeforeEach(func() {
		oldSpec = boshas.V1ApplySpec{PersistentDisk: 200}
		newSpec = boshas.V1ApplySpec{PersistentDisk: 301}
	})

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

var _ = Describe("ToStatusParams", func() {
	var (
		oldSpec, newSpec boshas.V1ApplySpec
	)

	BeforeEach(func() {
		oldSpec = boshas.V1ApplySpec{PersistentDisk: 200}
		newSpec = boshas.V1ApplySpec{PersistentDisk: 301}
	})

	Describe("JobState", func() {
		It("returns JSON serialized current spec that only includes persistent disk", func() {
			state, err := NewUpdateParams(oldSpec, newSpec).ToStatusParams().JobState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":200}`))
		})
	})

	Describe("JobNextState", func() {
		It("returns empty string because next state is never available", func() {
			state, err := NewUpdateParams(oldSpec, newSpec).ToStatusParams().JobNextState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(""))
		})
	})
})

var _ = Describe("NewUpdateParams", func() {
	Describe("UpdatedPackages", func() {
		It("returns list of packages that changed or got added in lexical order", func() {
			oldPkgs := map[string]boshas.PackageSpec{
				"foo": boshas.PackageSpec{
					Name: "foo",
					Sha1: crypto.MustParseMultipleDigest("sha1:foosha1old"),
				},
				"bar": boshas.PackageSpec{
					Name: "bar",
					Sha1: crypto.MustParseMultipleDigest("sha1:barsha1"),
				},
			}

			newPkgs := map[string]boshas.PackageSpec{
				"foo": boshas.PackageSpec{
					Name: "foo",
					Sha1: crypto.MustParseMultipleDigest("sha1:foosha1new"),
				},
				"bar": boshas.PackageSpec{
					Name: "bar",
					Sha1: crypto.MustParseMultipleDigest("sha1:barsha1"),
				},
				"baz": boshas.PackageSpec{
					Name: "baz",
					Sha1: crypto.MustParseMultipleDigest("sha1:bazsha1"),
				},
			}

			oldSpec := boshas.V1ApplySpec{
				PackageSpecs: oldPkgs,
			}

			newSpec := boshas.V1ApplySpec{
				PackageSpecs: newPkgs,
			}

			params := NewUpdateParams(oldSpec, newSpec)

			Expect(params.UpdatedPackages()).To(Equal([]string{"baz", "foo"}))
		})
	})

	Describe("JobState", func() {
		It("returns JSON serialized current spec that only includes persistent disk", func() {
			oldSpec := boshas.V1ApplySpec{PersistentDisk: 200}
			newSpec := boshas.V1ApplySpec{PersistentDisk: 301}
			params := NewUpdateParams(oldSpec, newSpec)

			state, err := params.JobState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":200}`))
		})
	})

	Describe("JobNextState", func() {
		It("returns JSON serialized future spec that only includes persistent disk", func() {
			oldSpec := boshas.V1ApplySpec{PersistentDisk: 200}
			newSpec := boshas.V1ApplySpec{PersistentDisk: 301}
			params := NewUpdateParams(oldSpec, newSpec)

			state, err := params.JobNextState()
			Expect(err).ToNot(HaveOccurred())
			Expect(state).To(Equal(`{"persistent_disk":301}`))
		})
	})
})
