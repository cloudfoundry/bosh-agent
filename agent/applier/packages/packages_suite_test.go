package packages_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPackages(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Packages Suite")
}
