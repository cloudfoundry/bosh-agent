package compiler_test

import (
	"errors"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakebc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	boshmodels "github.com/cloudfoundry/bosh-agent/agent/applier/models"
	fakepackages "github.com/cloudfoundry/bosh-agent/agent/applier/packages/fakes"
	fakecmdrunner "github.com/cloudfoundry/bosh-agent/agent/cmdrunner/fakes"
	. "github.com/cloudfoundry/bosh-agent/agent/compiler"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"

	fakecmd "github.com/cloudfoundry/bosh-utils/fileutil/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func init() {
	Describe("concreteCompiler", func() {
		var (
			compiler       Compiler
			compressor     *fakecmd.FakeCompressor
			blobstore      *fakeblobdelegator.FakeBlobstoreDelegator
			fs             *fakesys.FakeFileSystem
			runner         *fakecmdrunner.FakeFileLoggingCmdRunner
			packageApplier *fakepackages.FakeApplier
			packagesBc     *fakebc.FakeBundleCollection
			fakeClock      *fakebc.FakeClock
		)

		BeforeEach(func() {
			compressor = fakecmd.NewFakeCompressor()
			blobstore = &fakeblobdelegator.FakeBlobstoreDelegator{}
			fs = fakesys.NewFakeFileSystem()
			runner = fakecmdrunner.NewFakeFileLoggingCmdRunner()
			packageApplier = fakepackages.NewFakeApplier()
			packagesBc = fakebc.NewFakeBundleCollection()
			fakeClock = new(fakebc.FakeClock)

			compiler = NewConcreteCompiler(
				compressor,
				blobstore,
				fs,
				runner,
				FakeCompileDirProvider{Dir: "/fake-compile-dir"},
				packageApplier,
				packagesBc,
				fakeClock,
			)

			fs.MkdirAll("/fake-compile-dir", os.ModePerm)
			Expect(fs.WriteFileString("/tmp/compressed-compiled-package", "fake-contents")).ToNot(HaveOccurred())
		})

		Describe("Compile, when renaming the decompressed archive from temp to final dir", func() {
			var (
				bundle  *fakebc.FakeBundle
				pkg     Package
				pkgDeps []boshmodels.Package
			)

			BeforeEach(func() {
				bundle = packagesBc.FakeGet(boshmodels.LocalPackage{
					Name:    "pkg_name",
					Version: "pkg_version",
				})

				bundle.InstallPath = "/fake-dir/data/packages/pkg_name/pkg_version"
				bundle.EnablePath = "/fake-dir/packages/pkg_name"

				compressor.CompressFilesInDirTarballPath = "/tmp/compressed-compiled-package"

				pkg, pkgDeps = getCompileArgs()
			})

			It("succeeds if renaming eventually succeeds within the retry window", func() {
				callCounter := 0

				fs.RenameStub = func(oldPath, newPath string) error {
					Expect(fakeClock.SleepCallCount()).To(Equal(callCounter))
					callCounter++

					// TODO:  Define the total wait time to be, perhaps, 10s, and return a non-error only after that
					if callCounter <= 5 {
						return errors.New("can't perform filesystem rename")
					}

					return nil
				}

				_, _, err := compiler.Compile(pkg, pkgDeps)
				Expect(err).ToNot(HaveOccurred())

				Expect(fs.RenameOldPaths[0]).To(Equal("/fake-compile-dir/pkg_name-bosh-agent-unpack"))
				Expect(fs.RenameNewPaths[0]).To(Equal("/fake-compile-dir/pkg_name"))

				Expect(fakeClock.SleepCallCount()).To(Equal(5), "Should have called Sleep()")
				for i := 0; i < fakeClock.SleepCallCount(); i++ {
					Expect(fakeClock.SleepArgsForCall(i)).To(Equal(5 * time.Second))
				}
			})

			It("fails if renaming does not succeed within the retry window", func() {
				callCounter := 0

				fs.RenameStub = func(oldPath, newPath string) error {
					callCounter++
					return errors.New("can't perform filesystem rename")
				}

				startTime := time.Now()
				fakeClock.NowReturns(startTime)
				fakeClock.SinceReturns(CompileTimeout + time.Second)

				_, _, err := compiler.Compile(pkg, pkgDeps)
				Expect(err).To(MatchError(ContainSubstring("can't perform filesystem rename")))

				Expect(fakeClock.SinceCallCount()).To(Equal(1))
				Expect(fakeClock.SinceArgsForCall(0)).To(Equal(startTime))
				Expect(callCounter).To(Equal(1))
			})
		})
	})
}
