package script_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	fakeaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/script/drain"
	fakedrain "github.com/cloudfoundry/bosh-agent/agent/script/drain/fakes"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

var _ = Describe("ConcreteJobScriptProvider", func() {
	var (
		scriptProvider boshscript.ConcreteJobScriptProvider
	)

	BeforeEach(func() {
		runner := fakesys.NewFakeCmdRunner()
		fs := fakesys.NewFakeFileSystem()
		dirProvider := boshdir.NewProvider("/the/base/dir")
		scriptProvider = boshscript.NewConcreteJobScriptProvider(runner, fs, dirProvider, &fakeaction.FakeClock{})
	})

	Describe("NewScript", func() {
		It("returns script with relative job paths to the base directory", func() {
			script := scriptProvider.NewScript("myjob", "the-best-hook-ever")
			Expect(script.Tag()).To(Equal("myjob"))
			Expect(script.Path()).To(Equal("/the/base/dir/jobs/myjob/bin/the-best-hook-ever"))
		})
	})

	Describe("NewDrainScript", func() {
		It("returns drain script", func() {
			params := &fakedrain.FakeScriptParams{}
			script := scriptProvider.NewDrainScript("foo", params)
			Expect(script.Tag()).To(Equal("foo"))
			Expect(script.Path()).To(Equal("/the/base/dir/jobs/foo/bin/drain"))
			Expect(script.(boshdrain.ConcreteScript).Params()).To(Equal(params))
		})
	})
})
