package kickstart_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestKickstart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kickstart Suite")
}
