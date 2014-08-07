package matchers

import (
	"fmt"

	"github.com/onsi/gomega/types"
)

func MatchOneOf(matchers ...types.GomegaMatcher) *OneOfMatcher {
	if len(matchers) < 2 {
		panic(fmt.Sprintf("MatchOneOf requires at least two matchers. Got: %s", matchers))
	}
	return &OneOfMatcher{
		Matchers: matchers,
	}
}
