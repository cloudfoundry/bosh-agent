package applyspec_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestApplyspec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Apply Spec Suite")
}
