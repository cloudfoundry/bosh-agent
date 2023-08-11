package powershell_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPowershell(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Powershell Suite")
}
