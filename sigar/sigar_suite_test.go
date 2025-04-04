package sigar_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sigar Suite")
}
