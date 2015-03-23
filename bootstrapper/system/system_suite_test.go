package system_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBootstrapperSystem(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrapper System Suite")
}
