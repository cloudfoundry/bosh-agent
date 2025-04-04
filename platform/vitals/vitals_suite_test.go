package vitals_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVitals(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vitals Suite")
}
