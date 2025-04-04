package udevdevice_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUdevdevice(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "udev Device Suite")
}
