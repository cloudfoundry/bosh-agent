package cdrom_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCdrom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CD-ROM Suite")
}
