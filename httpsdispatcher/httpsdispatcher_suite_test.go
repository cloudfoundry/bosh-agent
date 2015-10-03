package httpsdispatcher_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"testing"
)

func TestHttpsdispatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Httpsdispatcher Suite")
}
