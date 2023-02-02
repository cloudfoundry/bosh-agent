package logstarprovider

import (
	"errors"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
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

					Expect(copier.FilteredCopyToTempDir).To(Equal("/fake/dir/sys/log"))
				})
			})

			Context("agent logs", func() {
				It("uses the correct logs dir", func() {
					_, err := provider.Get("agent", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredCopyToTempDir).To(Equal("/fake/dir/bosh/log"))
				})
			})
		})

		Describe("filters", func() {
			Context("job logs", func() {
				BeforeEach(func() {
					Expect(copier.FilteredCopyToTempFilters).To(BeEmpty())
				})

				It("uses the filters provided", func() {
					_, err := provider.Get("job", []string{"foo", "bar"})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredCopyToTempFilters).To(ConsistOf("foo", "bar"))
				})

				It("uses the default filters when none are provided", func() {
					_, err := provider.Get("job", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredCopyToTempFilters).To(ConsistOf("**/*"))
				})
			})

			Context("agent logs", func() {
				It("uses the filters provided", func() {
					_, err := provider.Get("agent", []string{"foo", "bar"})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredCopyToTempFilters).To(ConsistOf("foo", "bar"))
				})

				It("uses the default filters when none are provided", func() {
					_, err := provider.Get("agent", []string{})
					Expect(err).NotTo(HaveOccurred())

					Expect(copier.FilteredCopyToTempFilters).To(ConsistOf("**/*"))
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
						copier.FilteredCopyToTempError = errors.New("plagiarization")
					})

					It("returns an error if the copier returns an error", func() {
						_, err := provider.Get("job", []string{})
						Expect(err).To(MatchError(ContainSubstring("Copying filtered files to temp directory")))
						Expect(err).To(MatchError(ContainSubstring("plagiarization")))
					})
				})

				It("cleans up temp dir", func() {
					copier.FilteredCopyToTempTempDir = "/tmp/dir"
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
			provider.CleanUp("/tmp/logs.tar")
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
