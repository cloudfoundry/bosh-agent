package installer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBootstrapper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrapper Installer Suite")
}
