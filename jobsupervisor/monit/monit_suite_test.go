package monit_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"testing"
)

func TestMonit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Monit Suite")
}
