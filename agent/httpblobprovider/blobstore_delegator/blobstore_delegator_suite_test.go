package blobstore_delegator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBlobstoreDelegator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BlobstoreDelegator Suite")
}
