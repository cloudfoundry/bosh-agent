package drain_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/drain"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesys "github.com/cloudfoundry/bosh-agent/system/fakes"
)

func init() {
	Describe("Testing with Ginkgo", func() {
		It("new drain script", func() {

			runner := fakesys.NewFakeCmdRunner()
			fs := fakesys.NewFakeFileSystem()
			dirProvider := boshdir.NewDirectoriesProvider("/var/vcap")

			scriptProvider := NewConcreteDrainScriptProvider(runner, fs, dirProvider)
			drainScript := scriptProvider.NewDrainScript("foo")

			Expect(drainScript.Path()).To(Equal("/var/vcap/jobs/foo/bin/drain"))
		})
	})
}
