package matchers_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"

	"github.com/onsi/gomega/types"
	"github.com/onsi/gomega/internal/fakematcher"

	. "github.com/cloudfoundry/bosh-agent/matchers"
)

var _ = Describe("matchers", func() {

	var _ = Describe("Match", func() {

		Context("when no sub-matchers match", func() {
			var fakematcher1 = &fakematcher.FakeMatcher{
				MatchesToReturn: false,
				ErrToReturn: nil,
			}
			var fakematcher2 = &fakematcher.FakeMatcher{
				MatchesToReturn: false,
				ErrToReturn: nil,
			}
			var matchers = []types.GomegaMatcher{fakematcher1, fakematcher2}
			var oneOf = OneOfMatcher{Matchers: matchers}

			It("calls Match on each sub-matcher", func() {
				success, err := oneOf.Match("Fake Test Value")

				Expect(success).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())

				Expect(fakematcher1.ReceivedActual).To(Equal("Fake Test Value"))
				Expect(fakematcher2.ReceivedActual).To(Equal("Fake Test Value"))
			})
		})

		Context("when at least one sub-matcher matches", func() {
			var fakematcher1 = &fakematcher.FakeMatcher{
				MatchesToReturn: false,
				ErrToReturn: nil,
			}
			var fakematcher2 = &fakematcher.FakeMatcher{
				MatchesToReturn: true,
				ErrToReturn: nil,
			}
			var fakematcher3 = &fakematcher.FakeMatcher{
				MatchesToReturn: false,
				ErrToReturn: nil,
			}
			var matchers = []types.GomegaMatcher{fakematcher1, fakematcher2, fakematcher3}
			var oneOf = OneOfMatcher{Matchers: matchers}

			It("calls Match on each sub-matcher until a match is found", func() {
				success, err := oneOf.Match("Fake Test Value")

				Expect(success).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Expect(fakematcher1.ReceivedActual).To(Equal("Fake Test Value"))
				Expect(fakematcher2.ReceivedActual).To(Equal("Fake Test Value"))
				Expect(fakematcher3.ReceivedActual).To(BeNil())
			})
		})

		Context("when at least one sub-matcher errors", func() {
			var error = fmt.Errorf("Fake Error")
			var fakematcher1 = &fakematcher.FakeMatcher{
				MatchesToReturn: false,
				ErrToReturn: nil,
			}
			var fakematcher2 = &fakematcher.FakeMatcher{
				MatchesToReturn: false,
				ErrToReturn: error,
			}
			var fakematcher3 = &fakematcher.FakeMatcher{
				MatchesToReturn: true,
				ErrToReturn: nil,
			}
			var matchers = []types.GomegaMatcher{fakematcher1, fakematcher2, fakematcher3}
			var oneOf = OneOfMatcher{Matchers: matchers}

			It("calls Match on each sub-matcher until an error is returned", func() {
				success, err := oneOf.Match("Fake Test Value")

				Expect(success).To(BeFalse())
				Expect(err).To(Equal(error))

				Expect(fakematcher1.ReceivedActual).To(Equal("Fake Test Value"))
				Expect(fakematcher2.ReceivedActual).To(Equal("Fake Test Value"))
				Expect(fakematcher3.ReceivedActual).To(BeNil())
			})
		})
	})

	var _ = Describe("FailureMessage", func() {
		var fakematcher = &fakematcher.FakeMatcher{
			MatchesToReturn: false,
			ErrToReturn: nil,
		}
		var matchers = []types.GomegaMatcher{fakematcher, fakematcher}
		var oneOf = OneOfMatcher{Matchers: matchers}

		It("concatonates the failure message of all matchers", func() {
			expectedMessage := "positive: Fake Test Value\n --OR-- \npositive: Fake Test Value"

			msg := oneOf.FailureMessage("Fake Test Value")

			Expect(msg).To(Equal(expectedMessage))
		})
	})

	var _ = Describe("NegatedFailureMessage", func() {
		var fakematcher = &fakematcher.FakeMatcher{
			MatchesToReturn: false,
			ErrToReturn: nil,
		}
		var matchers = []types.GomegaMatcher{fakematcher, fakematcher}
		var oneOf = OneOfMatcher{Matchers: matchers}

		It("concatonates the failure message of all matchers", func() {
			expectedMessage := "negative: Fake Test Value\n --OR-- \nnegative: Fake Test Value"

			msg := oneOf.NegatedFailureMessage("Fake Test Value")

			Expect(msg).To(Equal(expectedMessage))
		})
	})
})
