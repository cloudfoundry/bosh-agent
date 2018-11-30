package bundlecollection_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	"github.com/cloudfoundry/bosh-agent/agent/tarpath/tarpathfakes"
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
		err := fs.MkdirAll(path, 0750)
		Expect(err).ToNot(HaveOccurred())

		err = fs.WriteFileString("/source-path/config.go", "package go")
		Expect(err).ToNot(HaveOccurred())

		return path
	}

	BeforeEach(func() {
		sourcePath = createSourcePath()
	})

	Describe("InstallWithoutContents", func() {
		It("installs the bundle at the given path with the correct permissions", func() {
			path, err := fileBundle.InstallWithoutContents()
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(installPath))

			fileStats := fs.GetFileTestStat(installPath)
			Expect(fileStats).ToNot(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
			Expect(fileStats.FileMode).To(Equal(os.FileMode(0750)))
			Expect(fileStats.Username).To(Equal("root"))
			Expect(fileStats.Groupname).To(Equal("vcap"))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
		})

		It("return error when bundle cannot be installed", func() {
			fs.MkdirAllError = errors.New("fake-mkdirall-error")

			_, err := fileBundle.InstallWithoutContents()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mkdirall-error"))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("return error when chwon fails", func() {
			fs.ChownErr = errors.New("chown failed")

			_, err := fileBundle.InstallWithoutContents()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("chown failed"))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("sets correct permissions on install path", func() {
			fs.Chmod(sourcePath, os.FileMode(0700))

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat(installPath)
			Expect(fileStats).ToNot(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
			Expect(fileStats.FileMode).To(Equal(os.FileMode(0750)))
			Expect(fileStats.Username).To(Equal("root"))
			Expect(fileStats.Groupname).To(Equal("vcap"))
		})

		It("is idempotent", func() {
			_, err := fileBundle.InstallWithoutContents()
			Expect(err).NotTo(HaveOccurred())

			path, err := fileBundle.InstallWithoutContents()
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(installPath))
		})
	})

	Describe("Install", func() {
		It("installs the bundle from source at the given path", func() {
			fakeCompressor.DecompressFileToDirCallBack = func() {
				decompressPath := fakeCompressor.DecompressFileToDirDirs[len(fakeCompressor.DecompressFileToDirDirs)-1]
				contents, err := fs.ReadFileString(filepath.Join(sourcePath, "config.go"))
				Expect(err).NotTo(HaveOccurred())
				fs.WriteFileString(filepath.Join(decompressPath, "config.go"), contents)
			}

			path, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(installPath))

			opts := fakeCompressor.DecompressFileToDirOptions[0]
			Expect(opts.PathInArchive).To(Equal(""))
			Expect(opts.StripComponents).To(Equal(0))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue(), "Bundle not installed")

			contents, err := fs.ReadFileString(filepath.Join(path, "config.go"))
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal("package go"))
		})

		It("returns error when decompression fails", func() {
			fakeCompressor.DecompressFileToDirErr = errors.New("disaster")

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).To(MatchError(ContainSubstring("disaster")))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("returns an error if creation of install directory fails", func() {
			fs.MkdirAllError = errors.New("mkdir failed")

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).To(MatchError(ContainSubstring("mkdir failed")))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("returns an error if the detection fails", func() {
			fakeDetector.DetectReturns(false, errors.New("detect failed"))

			_, err := fileBundle.Install(sourcePath, "subdir")
			Expect(err).To(MatchError(ContainSubstring("detect failed")))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("returns an error if chown install directory fails", func() {
			fs.ChownErr = errors.New("chown failed")

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).To(MatchError(ContainSubstring("chown failed")))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("sets correct permissions on install path", func() {
			fs.Chmod(sourcePath, os.FileMode(0700))

			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat(installPath)
			Expect(fileStats).ToNot(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
			Expect(fileStats.FileMode).To(Equal(os.FileMode(0750)))
			Expect(fileStats.Username).To(Equal("root"))
			Expect(fileStats.Groupname).To(Equal("vcap"))
		})

		// Job bundles contain many jobs but we only want to extract a single one.
		Describe("extracting only part of the bundle", func() {
			It("detects the prefix to use", func() {
				fakeDetector.DetectReturns(true, nil)

				path, err := fileBundle.Install(sourcePath, "subdir")
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal(installPath))

				opts := fakeCompressor.DecompressFileToDirOptions[0]
				Expect(opts.PathInArchive).To(Equal("./subdir"))
				Expect(opts.StripComponents).To(Equal(2))
			})
		})
	})

	Describe("GetInstallPath", func() {
		It("returns the install path", func() {
			err := fs.MkdirAll(installPath, 0750)
			Expect(err).NotTo(HaveOccurred())

			actualInstallPath, err := fileBundle.GetInstallPath()
			Expect(err).NotTo(HaveOccurred())
			Expect(actualInstallPath).To(Equal(installPath))
		})

		It("returns error when install directory does not exist", func() {
			_, err := fileBundle.GetInstallPath()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("IsInstalled", func() {
		It("returns true when it is installed", func() {
			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
		})

		It("returns false when it is NOT installed", func() {
			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})
	})

	Describe("Enable", func() {
		Context("when bundle is installed", func() {
			BeforeEach(func() {
				_, err := fileBundle.Install(sourcePath, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the enable path", func() {
				actualEnablePath, err := fileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())
				Expect(actualEnablePath).To(Equal(enablePath))

				fileStats := fs.GetFileTestStat(enablePath)
				Expect(fileStats).NotTo(BeNil())
				Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeSymlink)))
				Expect(installPath).To(Equal(fileStats.SymlinkTarget))

				fileStats = fs.GetFileTestStat("/") // dir holding symlink
				Expect(fileStats).NotTo(BeNil())
				Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
				Expect(fileStats.FileMode).To(Equal(os.FileMode(0750)))
				Expect(fileStats.Username).To(Equal("root"))
				Expect(fileStats.Groupname).To(Equal("vcap"))
			})

			It("is idempotent", func() {
				_, err := fileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())

				_, err = fileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when bundle is not installed", func() {
			It("returns error", func() {
				_, err := fileBundle.Enable()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("bundle must be installed"))
			})

			It("does not add symlink", func() {
				_, err := fileBundle.Enable()
				Expect(err).To(HaveOccurred())

				fileStats := fs.GetFileTestStat(enablePath)
				Expect(fileStats).To(BeNil())
			})
		})

		Context("when enable dir cannot be created", func() {
			It("returns error", func() {
				_, err := fileBundle.Install(sourcePath, "")
				Expect(err).NotTo(HaveOccurred())
				fs.MkdirAllError = errors.New("fake-mkdirall-error")

				_, err = fileBundle.Enable()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-mkdirall-error"))
			})
		})

		Context("when bundle cannot be enabled", func() {
			It("returns error", func() {
				_, err := fileBundle.Install(sourcePath, "")
				Expect(err).NotTo(HaveOccurred())
				fs.SymlinkError = errors.New("fake-symlink-error")

				_, err = fileBundle.Enable()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-symlink-error"))
			})
		})
	})

	Describe("Disable", func() {
		It("is idempotent", func() {
			err := fileBundle.Disable()
			Expect(err).NotTo(HaveOccurred())

			err = fileBundle.Disable()
			Expect(err).NotTo(HaveOccurred())

			Expect(fs.FileExists(enablePath)).To(BeFalse())
		})

		Context("where the enabled path target is the same installed version", func() {
			BeforeEach(func() {
				_, err := fileBundle.Install(sourcePath, "")
				Expect(err).NotTo(HaveOccurred())

				_, err = fileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return error and removes the symlink", func() {
				err := fileBundle.Disable()
				Expect(err).NotTo(HaveOccurred())
				Expect(fs.FileExists(enablePath)).To(BeFalse())
			})

			It("returns error when bundle cannot be disabled", func() {
				fs.RemoveAllStub = func(_ string) error {
					return errors.New("fake-removeall-error")
				}

				err := fileBundle.Disable()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-removeall-error"))
			})
		})

		Context("where the enabled path target is a different installed version", func() {
			newerInstallPath := "/newer-install-path"

			BeforeEach(func() {
				_, err := fileBundle.Install(sourcePath, "")
				Expect(err).NotTo(HaveOccurred())

				_, err = fileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())

				newerFileBundle := NewFileBundle(
					newerInstallPath,
					enablePath,
					os.FileMode(0750),
					fs,
					fakeClock,
					fakeCompressor,
					fakeDetector,
					logger,
				)

				otherSourcePath := createSourcePath()
				_, err = newerFileBundle.Install(otherSourcePath, "")
				Expect(err).NotTo(HaveOccurred())

				_, err = newerFileBundle.Enable()
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return error and does not remove the symlink", func() {
				err := fileBundle.Disable()
				Expect(err).NotTo(HaveOccurred())

				fileStats := fs.GetFileTestStat(enablePath)
				Expect(fileStats).NotTo(BeNil())
				Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeSymlink)))
				Expect(newerInstallPath).To(Equal(fileStats.SymlinkTarget))
			})
		})

		Context("when the symlink cannot be read", func() {
			It("returns error because we cannot determine if bundle is enabled or disabled", func() {
				fs.ReadAndFollowLinkError = errors.New("fake-read-link-error")

				err := fileBundle.Disable()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-read-link-error"))
			})
		})
	})

	Describe("Uninstall", func() {
		It("removes the files from disk", func() {
			_, err := fileBundle.Install(sourcePath, "")
			Expect(err).NotTo(HaveOccurred())

			err = fileBundle.Uninstall()
			Expect(err).NotTo(HaveOccurred())

			Expect(fs.FileExists(installPath)).To(BeFalse())
		})

		It("is idempotent", func() {
			err := fileBundle.Uninstall()
			Expect(err).NotTo(HaveOccurred())

			err = fileBundle.Uninstall()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
