package cmdrunner_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCompiler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Command Runner Suite")
}
