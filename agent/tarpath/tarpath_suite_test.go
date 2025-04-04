package tarpath_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTarpath(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tarpath Suite")
}
