// +build !windows

package bundlecollection_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	"github.com/cloudfoundry/bosh-agent/agent/tarpath/tarpathfakes"
	fakefileutil "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
)

var _ = Describe("FileBundle", func() {
	var (
		fs             *fakesys.FakeFileSystem
		fakeClock      *fakes.FakeClock
		fakeCompressor *fakefileutil.FakeCompressor
		fakeDetector   *tarpathfakes.FakeDetector
		logger         boshlog.Logger
		sourcePath     string
		installPath    string
		enablePath     string
		fileBundle     FileBundle
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		fakeClock = new(fakes.FakeClock)
		fakeCompressor = new(fakefileutil.FakeCompressor)
		fakeDetector = new(tarpathfakes.FakeDetector)
		installPath = "/install-path"
		enablePath = "/enable-path"
		logger = boshlog.NewLogger(boshlog.LevelNone)
		fileBundle = NewFileBundle(
			installPath,
			enablePath,
			os.FileMode(0750),
			fs,
			fakeClock,
			fakeCompressor,
			fakeDetector,
			logger,
		)
	})

	createSourcePath := func() string {
		path := "/source-path"
		err := fs.MkdirAll(path, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		err = fs.WriteFileString("/source-path/config.go", "package go")
		Expect(err).ToNot(HaveOccurred())

		return path
	}

	BeforeEach(func() {
		sourcePath = createSourcePath()
	})

	It("uninstalls the bundle", func() {
		fakeCompressor.DecompressFileToDirCallBack = func() {
			decompressPath := fakeCompressor.DecompressFileToDirDirs[len(fakeCompressor.DecompressFileToDirDirs)-1]
			contents, err := fs.ReadFileString(filepath.Join(sourcePath, "config.go"))
			Expect(err).NotTo(HaveOccurred())
			fs.WriteFileString(filepath.Join(decompressPath, "config.go"), contents)
		}

		path, err := fileBundle.Install(sourcePath, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(path).To(Equal(installPath))

		installed, err := fileBundle.IsInstalled()
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeTrue(), "Bundle not installed")

		err = fileBundle.Uninstall()
		Expect(err).NotTo(HaveOccurred())

		installed, err = fileBundle.IsInstalled()
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeFalse(), "Bundle was not uninstalled")
	})
})
