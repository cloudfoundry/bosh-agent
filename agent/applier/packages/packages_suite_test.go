package packages_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPackages(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Packages Suite")
}
