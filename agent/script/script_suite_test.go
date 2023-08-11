package script_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestScriptRunner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Script Suite")
}
