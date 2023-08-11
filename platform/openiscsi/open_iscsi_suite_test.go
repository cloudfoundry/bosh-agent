package openiscsi_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestUdevdevice(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Open Iscsi Suite")
}
