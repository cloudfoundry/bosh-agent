package applier_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestApplier(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Applier Suite")
}
