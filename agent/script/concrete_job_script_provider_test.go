package script_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	fakeaction "github.com/cloudfoundry/bosh-agent/v2/agent/action/fakes"
	boshscript "github.com/cloudfoundry/bosh-agent/v2/agent/script"
	boshdrain "github.com/cloudfoundry/bosh-agent/v2/agent/script/drain"
	"github.com/cloudfoundry/bosh-agent/v2/agent/script/drain/drainfakes"
	"github.com/cloudfoundry/bosh-agent/v2/agent/script/scriptfakes"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

var _ = Describe("ConcreteJobScriptProvider", func() {
	var (
		logger         boshlog.Logger
		scriptProvider boshscript.ConcreteJobScriptProvider
		scriptEnv      map[string]string
	)

	BeforeEach(func() {
		runner := fakesys.NewFakeCmdRunner()
		fs := fakesys.NewFakeFileSystem()
		dirProvider := boshdir.NewProvider("/the/base/dir")
		logger = boshlog.NewLogger(boshlog.LevelNone)
		scriptProvider = boshscript.NewConcreteJobScriptProvider(
			runner,
			fs,
			dirProvider,
			&fakeaction.FakeClock{},
			logger,
		)
	})

	Describe("NewScript", func() {
		It("returns script with relative job paths to the base directory", func() {
			script := scriptProvider.NewScript("myjob", "the-best-hook-ever", scriptEnv)
			Expect(script.Tag()).To(Equal("myjob"))

			expPath := "/the/base/dir/jobs/myjob/bin/the-best-hook-ever" + boshscript.ScriptExt
			Expect(script.Path()).To(boshassert.MatchPath(expPath))
		})
	})

	Describe("NewDrainScript", func() {
		It("returns drain script", func() {
			params := &drainfakes.FakeScriptParams{}
			script := scriptProvider.NewDrainScript("foo", params)
			Expect(script.Tag()).To(Equal("foo"))

			expPath := "/the/base/dir/jobs/foo/bin/drain" + boshscript.ScriptExt
			Expect(script.Path()).To(boshassert.MatchPath(expPath))
			Expect(script.(boshdrain.ConcreteScript).Params()).To(Equal(params))
		})
	})

	Describe("NewParallelScript", func() {
		It("returns parallel script", func() {
			scripts := []boshscript.Script{&scriptfakes.FakeScript{}}
			script := scriptProvider.NewParallelScript("foo", scripts)
			Expect(script).To(Equal(boshscript.NewParallelScript("foo", scripts, logger)))
		})
	})
})
