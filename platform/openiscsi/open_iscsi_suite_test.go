package openiscsi_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUdevdevice(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Open Iscsi Suite")
}
