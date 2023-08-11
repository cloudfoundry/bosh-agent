package blobstore_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCascadingBlobstore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blobstore Suite")
}
