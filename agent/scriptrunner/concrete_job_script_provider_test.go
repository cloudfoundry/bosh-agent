package scriptrunner_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/scriptrunner"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

var _ = Describe("ConcreteJobScriptProvider", func() {
	It("produces script paths relative to the base directory", func() {
		runner := fakesys.NewFakeCmdRunner()
		fs := fakesys.NewFakeFileSystem()
		dirProvider := boshdir.NewProvider("/the/base/dir")

		scriptProvider := scriptrunner.NewConcreteJobScriptProvider(runner, fs, dirProvider)
		script := scriptProvider.Get("myjob", "the-best-hook-ever")
		Expect(script.Tag()).To(Equal("myjob"))
		Expect(script.Path()).To(Equal("/the/base/dir/jobs/myjob/bin/the-best-hook-ever"))
	})
})
