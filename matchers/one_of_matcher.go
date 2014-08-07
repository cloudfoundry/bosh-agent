package matchers

import (
	"github.com/onsi/gomega/types"
)

type OneOfMatcher struct {
	Matchers []types.GomegaMatcher
}

func (matcher *OneOfMatcher) Match(actual interface{}) (success bool, err error) {
	for _, submatcher := range matcher.Matchers {
		success, err = submatcher.Match(actual)
		if success || err != nil {
			return
		}
	}
	return
}

func (matcher *OneOfMatcher) FailureMessage(actual interface{}) (message string) {
	errorMsg := ""
	numMatchers := len(matcher.Matchers)
	for i := 0; i < numMatchers-1; i++ {
		errorMsg += matcher.Matchers[i].FailureMessage(actual)
		errorMsg += "\n --OR-- \n"
	}
	errorMsg += matcher.Matchers[numMatchers-1].FailureMessage(actual)

	return errorMsg
}

func (matcher *OneOfMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	errorMsg := ""
	numMatchers := len(matcher.Matchers)
	for i := 0; i < numMatchers-1; i++ {
		errorMsg += matcher.Matchers[i].NegatedFailureMessage(actual)
		errorMsg += "\n --OR-- \n"
	}
	errorMsg += matcher.Matchers[numMatchers-1].NegatedFailureMessage(actual)

	return errorMsg
}
