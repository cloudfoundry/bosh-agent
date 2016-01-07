package pkg_test

import (
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/platform/pkg"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UbuntuPkgManager", func() {
	var (
		cmdRunner  *fakesys.FakeCmdRunner
		pkgManager Manager
	)

	BeforeEach(func() {
		cmdRunner = fakesys.NewFakeCmdRunner()
		pkgManager = NewUbuntuPkgManager(cmdRunner)
	})

	Describe("RemovePackage", func() {
		It("removes specified package", func() {
			pkgManager.RemovePackage("dummy-compiler")
			Expect(len(cmdRunner.RunCommands)).To(Equal(2))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"apt-get", "-y", "remove", "dummy-compiler"}))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"apt-get", "-y", "autoremove"}))
		})
	})
})
