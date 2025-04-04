package bundlecollection_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBundlecollection(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bundle Collection Suite")
}
