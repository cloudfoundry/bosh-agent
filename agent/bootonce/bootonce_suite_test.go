package bootonce_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBootonce(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootonce Suite")
}
