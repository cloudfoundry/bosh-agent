package dnsresolver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDNSresolver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DNS Resolver Suite")
}
