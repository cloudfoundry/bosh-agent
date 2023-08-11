package applyspec_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestApplyspec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Apply Spec Suite")
}
