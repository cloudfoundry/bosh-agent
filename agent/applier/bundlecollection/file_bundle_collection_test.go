package bundlecollection_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/v2/agent/tarpath"

	fakefileutil "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection/fakes"
)

type testBundle struct {
	Name    string
	Version string
}

func (s testBundle) BundleName() string    { return s.Name }
func (s testBundle) BundleVersion() string { return s.Version }

var _ = Describe("FileBundleCollection", func() {
	var (
		fs                   *fakesys.FakeFileSystem
		fakeClock            *fakes.FakeClock
		fakeCompressor       *fakefileutil.FakeCompressor
		logger               boshlog.Logger
		fileBundleCollection FileBundleCollection
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		fakeClock = new(fakes.FakeClock)
		logger = boshlog.NewLogger(boshlog.LevelNone)
		fileBundleCollection = NewFileBundleCollection(
			"/fake-collection-path/data",
			"/fake-collection-path",
			"fake-collection-name",
			os.FileMode(0750),
			fs,
			fakeClock,
			fakeCompressor,
			logger,
		)
	})

	Describe("Get", func() {
		It("returns the file bundle with sha1'd bundle version as the last segment in the path", func() {
			bundleDefinition := testBundle{
				Name:    "fake-bundle-name",
				Version: "fake-bundle-version",
			}

			fileBundle, err := fileBundleCollection.Get(bundleDefinition)
			Expect(err).NotTo(HaveOccurred())

			expectedBundle := NewFileBundle(
				"/fake-collection-path/data/fake-collection-name/fake-bundle-name/faf990988742db852eec285122b5c4e7180e7be5",
				"/fake-collection-path/fake-collection-name/fake-bundle-name",
				os.FileMode(0750),
				fs,
				fakeClock,
				fakeCompressor,
				tarpath.NewPrefixDetector(),
				logger,
			)

			Expect(fileBundle).To(Equal(expectedBundle))
		})

		Context("when definition is missing name", func() {
			It("returns error", func() {
				_, err := fileBundleCollection.Get(testBundle{Version: "fake-bundle-version"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Missing bundle name"))
			})
		})

		Context("when definition is missing version", func() {
			It("returns error", func() {
				_, err := fileBundleCollection.Get(testBundle{Name: "fake-bundle-name"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Missing bundle version"))
			})
		})
	})

	Describe("List", func() {
		installPath := "/fake-collection-path/data/fake-collection-name"
		enablePath := "/fake-collection-path/fake-collection-name"

		It("returns list of installed bundles", func() {
			fs.SetGlob(installPath+"/*/*", []string{
				installPath + "/fake-bundle-1-name/fake-bundle-1-version-1",
				installPath + "/fake-bundle-1-name/fake-bundle-1-version-2",
				installPath + "/fake-bundle-2-name/fake-bundle-2-version-1",
			})

			bundles, err := fileBundleCollection.List()
			Expect(err).ToNot(HaveOccurred())

			expectedBundles := []Bundle{
				NewFileBundle(
					installPath+"/fake-bundle-1-name/fake-bundle-1-version-1",
					enablePath+"/fake-bundle-1-name",
					os.FileMode(0750),
					fs,
					fakeClock,
					fakeCompressor,
					tarpath.NewPrefixDetector(),
					logger,
				),
				NewFileBundle(
					installPath+"/fake-bundle-1-name/fake-bundle-1-version-2",
					enablePath+"/fake-bundle-1-name",
					os.FileMode(0750),
					fs,
					fakeClock,
					fakeCompressor,
					tarpath.NewPrefixDetector(),
					logger,
				),
				NewFileBundle(
					installPath+"/fake-bundle-2-name/fake-bundle-2-version-1",
					enablePath+"/fake-bundle-2-name",
					os.FileMode(0750),
					fs,
					fakeClock,
					fakeCompressor,
					tarpath.NewPrefixDetector(),
					logger,
				),
			}

			Expect(bundles).To(Equal(expectedBundles))
		})

		It("returns error when glob fails to execute", func() {
			fs.GlobErr = errors.New("fake-glob-error")

			_, err := fileBundleCollection.List()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-glob-error"))
		})

		It("returns error when bundle cannot be built from matched path", func() {
			invalidPaths := []string{
				"",
				"/",
				"before-slash/",
				"/after-slash",
				"no-slash",
			}

			for _, path := range invalidPaths {
				fs.SetGlob(installPath+"/*/*", []string{path})
				_, err := fileBundleCollection.List()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Getting bundle: Missing bundle name"))
			}
		})
	})
})
