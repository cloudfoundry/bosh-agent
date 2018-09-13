package disk_test

import (
	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Protector", func() {
	Describe("CommandExists", func() {
		It("calls CommandExists of powershell runner, returns the result when true", func() {
			powershellRunner := fakes.NewFakeCmdRunner()
			powershellRunner.CommandExistsValue = true

			protector := &disk.Protector{
				Runner: powershellRunner,
			}

			Expect(protector.CommandExists()).To(BeTrue())
		})

		It("calls CommandExists of powershell runner, returns the result when false", func() {
			powershellRunner := fakes.NewFakeCmdRunner()
			powershellRunner.CommandExistsValue = false

			protector := &disk.Protector{
				Runner: powershellRunner,
			}

			Expect(protector.CommandExists()).To(BeFalse())
		})
	})
})
