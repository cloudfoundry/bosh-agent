package garden_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Integration Suite")
}
