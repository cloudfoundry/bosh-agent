package platform_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakedpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver/fakes"
	. "github.com/cloudfoundry/bosh-agent/platform"
	fakestats "github.com/cloudfoundry/bosh-agent/platform/stats/fakes"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("WindowsPlatform", func() {
	var (
		collector          *fakestats.FakeCollector
		fs                 *fakesys.FakeFileSystem
		cmdRunner          *fakesys.FakeCmdRunner
		dirProvider        boshdirs.Provider
		devicePathResolver *fakedpresolv.FakeDevicePathResolver
		platform           Platform

		options LinuxOptions
		logger  boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)

		collector = &fakestats.FakeCollector{}
		fs = fakesys.NewFakeFileSystem()
		cmdRunner = fakesys.NewFakeCmdRunner()
		dirProvider = boshdirs.NewProvider("/fake-dir")
		devicePathResolver = fakedpresolv.NewFakeDevicePathResolver()
		options = LinuxOptions{}
	})

	JustBeforeEach(func() {
		platform = NewWindowsPlatform(
			collector,
			fs,
			cmdRunner,
			dirProvider,
			devicePathResolver,
			logger,
		)
	})

	Describe("GetFileContentsFromCDROM", func() {
		It("reads file from D drive", func() {
			fs.WriteFileString("D:/env", "fake-contents")
			contents, err := platform.GetFileContentsFromCDROM("env")
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal([]byte("fake-contents")))
		})
	})
})
