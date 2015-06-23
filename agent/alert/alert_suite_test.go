package alert_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"testing"
)

func TestAlert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Alert Suite")
}
