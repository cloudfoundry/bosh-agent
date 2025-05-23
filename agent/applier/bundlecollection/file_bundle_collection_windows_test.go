package bundlecollection_test

import (
	"os"
	"path"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fakefileutil "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/agent/tarpath"
)

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
			`C:\fake-collection-path\data`,
			`C:\fake-collection-path`,
			`fake-collection-name`,
			os.FileMode(0750),
			fs,
			fakeClock,
			fakeCompressor,
			logger,
		)
	})

	Describe("Get", func() {
		It("returns the file bundle", func() {
			bundleDefinition := testBundle{
				Name:    "fake-bundle-name",
				Version: "fake-bundle-version",
			}

			fileBundle, err := fileBundleCollection.Get(bundleDefinition)
			Expect(err).NotTo(HaveOccurred())

			expectedBundle := NewFileBundle(
				`C:/fake-collection-path/data/fake-collection-name/fake-bundle-name/faf990988742db852eec285122b5c4e7180e7be5`,
				`C:/fake-collection-path/fake-collection-name/fake-bundle-name`,
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
		installPath := `C:\fake-collection-path\data\fake-collection-name`
		enablePath := `C:\fake-collection-path\fake-collection-name`

		It("returns list of installed bundles for windows style paths", func() {
			fs.SetGlob(cleanPath(installPath+`\*\*`), []string{
				installPath + `\fake-bundle-1-name\fake-bundle-1-version-1`,
				installPath + `\fake-bundle-1-name\fake-bundle-1-version-2`,
				installPath + `\fake-bundle-1-name\fake-bundle-2-version-1`,
			})

			bundles, err := fileBundleCollection.List()
			Expect(err).ToNot(HaveOccurred())

			expectedBundles := []Bundle{
				NewFileBundle(
					cleanPath(installPath+`\fake-bundle-1-name\fake-bundle-1-version-1`),
					cleanPath(enablePath+`\fake-bundle-1-name`),
					os.FileMode(0750),
					fs,
					fakeClock,
					fakeCompressor,
					tarpath.NewPrefixDetector(),
					logger,
				),
				NewFileBundle(
					cleanPath(installPath+`\fake-bundle-1-name\fake-bundle-1-version-2`),
					cleanPath(enablePath+`\fake-bundle-1-name`),
					os.FileMode(0750),
					fs,
					fakeClock,
					fakeCompressor,
					tarpath.NewPrefixDetector(),
					logger,
				),
				NewFileBundle(
					cleanPath(installPath+`\fake-bundle-1-name\fake-bundle-2-version-1`),
					cleanPath(enablePath+`\fake-bundle-1-name`),
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
	})
})

func cleanPath(name string) string {
	return path.Clean(filepath.ToSlash(name))
}
