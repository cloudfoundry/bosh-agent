package logstarprovider

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHttpBlobProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LogsTarProvider Suite")
}
