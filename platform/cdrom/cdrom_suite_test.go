package cdrom_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCdrom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CD-ROM Suite")
}
