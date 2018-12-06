// +build windows

package bundlecollection_test

import (
	"github.com/cloudfoundry/bosh-agent/agent/tarpath/tarpathfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"

	. "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	fakefileutil "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

//go:generate counterfeiter -o fakes/fake_clock.go ../../../vendor/code.cloudfoundry.org/clock Clock

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

		err := fs.MkdirAll("/D/data", os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		err = fs.MkdirAll("/D/var/vcap", os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		err = fs.Symlink("/D/data", "/C/var/vcap/data")
		Expect(err).ToNot(HaveOccurred())

		err = fs.MkdirAll("/C/var/vcap/data/jobs", os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		err = fs.MkdirAll("/C/var/vcap/jobs", os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		installPath = "/C/var/vcap/data/jobs/job_name"
		enablePath = "/C/var/vcap/jobs/job_name"
		logger = boshlog.NewLogger(boshlog.LevelNone)

		fakeCompressor = new(fakefileutil.FakeCompressor)
		fakeDetector = &tarpathfakes.FakeDetector{}

		fileBundle = NewFileBundle(installPath, enablePath, os.FileMode(0750), fs, fakeClock, fakeCompressor, fakeDetector, logger)
	})

	createSourcePath := func() string {
		path := "/D/source-path"
		err := fs.MkdirAll(path, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		return path
	}

	BeforeEach(func() {
		sourcePath = createSourcePath()
	})

	Describe("Disable", func() {
		Context("where the enabled path target is the same installed version", func() {
			BeforeEach(func() {
				_, err := fileBundle.Install(sourcePath, "")
				Expect(err).NotTo(HaveOccurred())

				_, err = fileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())

				Expect(fs.FileExists(enablePath)).To(BeTrue())
			})
			It("does not return error and removes the symlink", func() {
				err := fileBundle.Disable()
				Expect(err).NotTo(HaveOccurred())
				Expect(fs.FileExists(enablePath)).To(BeFalse())
			})
		})
	})
})
