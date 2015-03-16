package kickstarter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestKickstarter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kickstarter Suite")
}
