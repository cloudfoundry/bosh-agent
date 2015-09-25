package drain_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	. "github.com/cloudfoundry/bosh-agent/agent/drain"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

func init() {
	Describe("Testing with Ginkgo", func() {
		It("new drain script", func() {

			runner := fakesys.NewFakeCmdRunner()
			fs := fakesys.NewFakeFileSystem()
			dirProvider := boshdir.NewProvider("/var/vcap")

			scriptProvider := NewConcreteScriptProvider(runner, fs, dirProvider, &fakes.FakeClock{})
			script := scriptProvider.NewScript("foo")

			Expect(script.Path()).To(Equal("/var/vcap/jobs/foo/bin/drain"))
		})
	})
}
