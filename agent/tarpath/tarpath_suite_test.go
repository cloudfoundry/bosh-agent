package tarpath_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTarpath(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tarpath Suite")
}
