package bundlecollection_test

import (
	. "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/v2/agent/tarpath/tarpathfakes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"errors"
	"os"

	"time"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection/fakes"
	fakefileutil "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("FileBundle uninstallation", func() {
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
		fakeDetector = &tarpathfakes.FakeDetector{}
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
		return path
	}

	BeforeEach(func() {
		sourcePath = createSourcePath()
	})

	Describe("Uninstall", func() {
		It("succeeds when the first five calls to RemoveAll fails", func() {
			callCounter := 0

			fs.RemoveAllStub = func(path string) error {
				callCounter++

				// TODO:  Define the total wait time to be, perhaps, 10s, and return a non-error only after that
				if callCounter <= 5 {
					return errors.New("Can't remove from the filesystem")
				}

				return nil
			}

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())

			err = fileBundle.Uninstall()
			Expect(err).NotTo(HaveOccurred())

			Expect(fs.FileExists(installPath)).To(BeFalse())

			Expect(fakeClock.SleepCallCount()).To(BeNumerically(">", 0), "Should have called Sleep()")
		})

		It("fails when repeatedly attempting RemoveAll times out", func() {
			expectedError := "Can't remove from the filesystem"
			fsRemoveAllCount := 0

			fs.RemoveAllStub = func(path string) error {
				fsRemoveAllCount++
				return errors.New(expectedError)
			}

			expectedStartTime := time.Unix(1000, 0)
			failingRemoveAlls := 5

			fakeClock.NowReturns(expectedStartTime)
			fakeClock.SinceReturns(1 * time.Second)
			fakeClock.SinceReturnsOnCall(failingRemoveAlls, BundleSetupTimeout+(1*time.Second))

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())

			err = fileBundle.Uninstall()
			Expect(err).To(MatchError(expectedError))

			Expect(fakeClock.SinceCallCount()).To(Equal(failingRemoveAlls + 1))
			for i := 0; i < failingRemoveAlls; i++ {
				Expect(fakeClock.SinceArgsForCall(i)).To(Equal(expectedStartTime))
			}
			Expect(fsRemoveAllCount).To(Equal(failingRemoveAlls))
		})
	})
})
