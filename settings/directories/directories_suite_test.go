package directories_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDirectories(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Directories Suite")
}
