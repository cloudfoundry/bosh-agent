package logstarprovider

import (
	"errors"
	"runtime"

	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"

	fakecmd "github.com/cloudfoundry/bosh-utils/fileutil/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogsTarProvider", func() {
	var (
		compressor  *fakecmd.FakeCompressor
		copier      *fakecmd.FakeCopier
		dirProvider boshdirs.Provider

		provider LogsTarProvider
	)

	BeforeEach(func() {
		compressor = fakecmd.NewFakeCompressor()
		dirProvider = boshdirs.NewProvider("/fake/dir")
		copier = fakecmd.NewFakeCopier()

		provider = NewLogsTarProvider(compressor, copier, dirProvider)
	})

	Describe("Get", func() {
		Describe("logsDir", func() {
			BeforeEach(func() {
				Expect(copier.FilteredCopyToTempDir).To(BeZero())
			})

			Context("job logs", func() {
				It("uses the correct logs dir", func() {
					_, err := provider.Get("job", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempDirs[0].Dir).To(boshassert.MatchPath(dirProvider.LogsDir()))
				})
			})

			Context("agent logs", func() {
				It("uses the correct logs dir", func() {
					_, err := provider.Get("agent", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempDirs[0].Dir).To(boshassert.MatchPath(dirProvider.AgentLogsDir()))
				})
			})

			Context("system logs", func() {
				It("uses the correct logs dir", func() {
					_, err := provider.Get("system", []string{})
					Expect(err).NotTo(HaveOccurred())

					if runtime.GOOS == "linux" {
						Expect(copier.FilteredMultiCopyToTempDirs[0].Dir).To(boshassert.MatchPath("/var/log"))
					} else {
						Expect(len(copier.FilteredMultiCopyToTempDirs)).To(Equal(0))
					}
				})
			})

			Context("multiple logs", func() {
				It("uses the correct logs dirs", func() {
					_, err := provider.Get("job,agent,system", []string{})
					Expect(err).NotTo(HaveOccurred())

					if runtime.GOOS == "linux" {
						Expect(len(copier.FilteredMultiCopyToTempDirs)).To(Equal(3))

						Expect(copier.FilteredMultiCopyToTempDirs[0].Dir).To(boshassert.MatchPath("/fake/dir/sys/log"))
						Expect(copier.FilteredMultiCopyToTempDirs[1].Dir).To(boshassert.MatchPath("/fake/dir/bosh/log"))
						Expect(copier.FilteredMultiCopyToTempDirs[2].Dir).To(boshassert.MatchPath("/var/log"))
					} else {
						Expect(len(copier.FilteredMultiCopyToTempDirs)).To(Equal(2))

						Expect(copier.FilteredMultiCopyToTempDirs[0].Dir).To(boshassert.MatchPath("/fake/dir/sys/log"))
						Expect(copier.FilteredMultiCopyToTempDirs[1].Dir).To(boshassert.MatchPath("/fake/dir/bosh/log"))
					}
				})
			})
		})

		Describe("filters", func() {
			BeforeEach(func() {
				Expect(copier.FilteredMultiCopyToTempFilters).To(BeEmpty())
			})

			Context("job logs", func() {
				It("uses the filters provided", func() {
					_, err := provider.Get("job", []string{"foo", "bar"})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("foo", "bar"))
				})

				It("uses the default filters when none are provided", func() {
					_, err := provider.Get("job", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("**/*"))
				})
			})

			Context("agent logs", func() {
				It("uses the filters provided", func() {
					_, err := provider.Get("agent", []string{"foo", "bar"})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("foo", "bar"))
				})

				It("uses the default filters when none are provided", func() {
					_, err := provider.Get("agent", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("**/*"))
				})
			})

			Context("system logs", func() {
				It("uses the filters provided", func() {
					_, err := provider.Get("system", []string{"foo", "bar"})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("foo", "bar"))
				})

				It("uses the default filters when none are provided", func() {
					_, err := provider.Get("system", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("**/*"))
				})
			})

			Context("multiple log types", func() {
				It("uses the filters provided, just as it does with one log type", func() {
					_, err := provider.Get("system,agent,job", []string{"foo", "bar"})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("foo", "bar"))
				})

				It("uses the default filters when none are provided", func() {
					_, err := provider.Get("agent,system,job", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredMultiCopyToTempFilters).To(ConsistOf("**/*"))
				})
			})

			Context("invalid log types", func() {
				It("returns an error", func() {
					_, err := provider.Get("lincoln", []string{})
					Expect(err).To(MatchError("Invalid log type"))
				})
			})

			Context("copying", func() {
				Context("error", func() {
					BeforeEach(func() {
						copier.FilteredMultiCopyToTempError = errors.New("plagiarization")
					})

					It("returns an error if the copier returns an error", func() {
						_, err := provider.Get("job", []string{})
						Expect(err).To(MatchError(ContainSubstring("Copying filtered files to temp directory")))
						Expect(err).To(MatchError(ContainSubstring("plagiarization")))
					})
				})

				It("cleans up temp dir", func() {
					copier.FilteredMultiCopyToTempDir = "/tmp/dir"
					Expect(copier.CleanUpTempDir).To(BeZero())

					_, err := provider.Get("job", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.CleanUpTempDir).To(Equal("/tmp/dir"))
				})
			})

			Context("compressing", func() {
				Context("error", func() {
					BeforeEach(func() {
						compressor.CompressFilesInDirErr = errors.New("squish")
					})

					It("returns an error if the compressor returns an error", func() {
						_, err := provider.Get("job", []string{})
						Expect(err).To(MatchError(ContainSubstring("Making logs tarball")))
						Expect(err).To(MatchError(ContainSubstring("squish")))
					})
				})

				It("returns the tarball path", func() {
					compressor.CompressFilesInDirTarballPath = "/tmp/logs.tar"

					tarballPath, err := provider.Get("job", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(tarballPath).To(Equal("/tmp/logs.tar"))
				})
			})
		})
	})

	Describe("Cleanup", func() {
		It("invokes the compressor's cleanup function", func() {
			Expect(compressor.CleanUpTarballPath).To(BeZero())
			_ = provider.CleanUp("/tmp/logs.tar")
			Expect(compressor.CleanUpTarballPath).To(Equal("/tmp/logs.tar"))
		})

		Context("error", func() {
			BeforeEach(func() {
				compressor.CleanUpErr = errors.New("big mess")
			})

			It("returns an error if the copier returns an error", func() {
				err := provider.CleanUp("")
				Expect(err).To(MatchError(ContainSubstring("big mess")))
			})
		})
	})
})
