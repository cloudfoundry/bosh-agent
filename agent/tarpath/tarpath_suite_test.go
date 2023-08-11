package tarpath_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTarpath(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tarpath Suite")
}
