package packages_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"

	boshbc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	. "github.com/cloudfoundry/bosh-agent/agent/applier/packages"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	fakecmd "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("compiledPackageApplierProvider", func() {
	var (
		blobstore  *fakeblobdelegator.FakeBlobstoreDelegator
		compressor *fakecmd.FakeCompressor
		fs         *fakesys.FakeFileSystem
		fakeClock  *fakes.FakeClock
		logger     boshlog.Logger
		provider   ApplierProvider
	)

	BeforeEach(func() {
		blobstore = &fakeblobdelegator.FakeBlobstoreDelegator{}
		compressor = fakecmd.NewFakeCompressor()
		fs = fakesys.NewFakeFileSystem()
		fakeClock = new(fakes.FakeClock)
		logger = boshlog.NewLogger(boshlog.LevelNone)
		provider = NewCompiledPackageApplierProvider(
			"fake-install-path",
			"fake-root-enable-path",
			"fake-job-specific-enable-path",
			"fake-name",
			blobstore,
			compressor,
			fs,
			fakeClock,
			logger,
		)
	})

	Describe("Root", func() {
		It("returns package applier that is configured to update system wide packages", func() {
			expected := NewCompiledPackageApplier(
				boshbc.NewFileBundleCollection(
					"fake-install-path",
					"fake-root-enable-path",
					"fake-name",
					os.FileMode(0755),
					fs,
					fakeClock,
					compressor,
					logger,
				),
				true,
				blobstore,
				fs,
				logger,
			)
			Expect(provider.Root()).To(Equal(expected))
		})
	})

	Describe("JobSpecific", func() {
		It("returns package applier that is configured to only update job specific packages", func() {
			expected := NewCompiledPackageApplier(
				boshbc.NewFileBundleCollection(
					"fake-install-path",
					"fake-job-specific-enable-path/fake-job-name",
					"fake-name",
					os.FileMode(0755),
					fs,
					fakeClock,
					compressor,
					logger,
				),

				// Should not operate as owner because keeping-only job specific packages
				// should not delete packages that could potentially be used by other jobs
				false,

				blobstore,
				fs,
				logger,
			)
			Expect(provider.JobSpecific("fake-job-name")).To(Equal(expected))
		})
	})
})
