package blobstore_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCascadingBlobstore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blobstore Suite")
}
