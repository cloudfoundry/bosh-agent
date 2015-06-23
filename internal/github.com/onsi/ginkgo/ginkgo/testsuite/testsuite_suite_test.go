package testsuite_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"
	. "github.com/onsi/ginkgo"

	"testing"
)

func TestTestsuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testsuite Suite")
}
