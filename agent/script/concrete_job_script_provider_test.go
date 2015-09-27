package script_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

var _ = Describe("ConcreteJobScriptProvider", func() {
	It("produces script paths relative to the base directory", func() {
		runner := fakesys.NewFakeCmdRunner()
		fs := fakesys.NewFakeFileSystem()
		dirProvider := boshdir.NewProvider("/the/base/dir")

		scriptProvider := boshscript.NewConcreteJobScriptProvider(runner, fs, dirProvider)
		script := scriptProvider.Get("myjob", "the-best-hook-ever")
		Expect(script.Tag()).To(Equal("myjob"))
		Expect(script.Path()).To(Equal("/the/base/dir/jobs/myjob/bin/the-best-hook-ever"))
	})
})
