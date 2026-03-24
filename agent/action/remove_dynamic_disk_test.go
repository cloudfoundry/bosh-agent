package action

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
)

var _ = Describe("RemoveDynamicDiskAction", func() {
	var (
		action   RemoveDynamicDiskAction
		platform *platformfakes.FakePlatform
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		action = NewRemoveDynamicDiskAction(platform)
	})

	It("cleans up dynamic disk", func() {
		result, err := action.Run("diskCID")

		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]string{}))
		Expect(platform.CleanupDynamicDiskCallCount()).To(Equal(1))
		Expect(platform.CleanupDynamicDiskArgsForCall(0)).To(Equal("diskCID"))
	})

	Context("when cleaning up dynamic disk fails", func() {
		BeforeEach(func() {
			platform.CleanupDynamicDiskReturns(errors.New("Could not setup"))
		})

		It("should raise error", func() {
			_, err := action.Run("diskCID")
			Expect(err).To(HaveOccurred())
		})
	})
})
