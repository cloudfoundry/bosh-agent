package vitals_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVitals(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vitals Suite")
}
