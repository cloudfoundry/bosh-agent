package monit_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestMonit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Monit Suite")
}
