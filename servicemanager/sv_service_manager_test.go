package servicemanager_test

import (
	"runtime"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/servicemanager"
)

var _ = Describe("svServiceManager", func() {
	var (
		serviceManager servicemanager.ServiceManager
		runner         *fakesys.FakeCmdRunner
		fs             *fakesys.FakeFileSystem
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("Not implemented on Windows")
		}

		fs = fakesys.NewFakeFileSystem()
		runner = fakesys.NewFakeCmdRunner()
		serviceManager = servicemanager.NewSvServiceManager(fs, runner)
	})

	Describe("Setup", func() {
		It("creates a symlink between /etc/service/monit and /etc/sv/monit", func() {
			err := fs.MkdirAll("/etc/sv/monit", 0750)
			Expect(err).NotTo(HaveOccurred())

			err = serviceManager.Setup("monit")
			Expect(err).NotTo(HaveOccurred())

			target, err := fs.ReadAndFollowLink("/etc/service/monit")
			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(Equal("/etc/sv/monit"))
		})
	})
})
