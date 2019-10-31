package jobs_test

import (
	"errors"
	"io"
	"os"

	. "github.com/cloudfoundry/bosh-agent/agent/applier/jobs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/settings/directories"

	boshbc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	fakebc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	fakepackages "github.com/cloudfoundry/bosh-agent/agent/applier/packages/fakes"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("renderedJobApplier", func() {
	var (
		jobsBc                 *fakebc.FakeBundleCollection
		jobSupervisor          *fakejobsuper.FakeJobSupervisor
		packageApplierProvider *fakepackages.FakeApplierProvider
		blobstore              *fakeblobdelegator.FakeBlobstoreDelegator
		fs                     *fakesys.FakeFileSystem
		applier                Applier
		fixPermissions         *fakeFixer
	)

	BeforeEach(func() {
		jobsBc = fakebc.NewFakeBundleCollection()
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		packageApplierProvider = fakepackages.NewFakeApplierProvider()
		blobstore = &fakeblobdelegator.FakeBlobstoreDelegator{}
		fs = fakesys.NewFakeFileSystem()
		logger := boshlog.NewLogger(boshlog.LevelNone)
		dirProvider := directories.NewProvider("/fakebasedir")
		fixPermissions = &fakeFixer{}

		applier = NewRenderedJobApplier(
			blobstore,
			dirProvider,
			jobsBc,
			jobSupervisor,
			packageApplierProvider,
			fixPermissions.Fix,
			fs,
			logger,
		)
	})

	Describe("Prepare & Apply", func() {
		var (
			job    models.Job
			bundle *fakebc.FakeBundle
		)

		BeforeEach(func() {
			job, bundle = buildJob(jobsBc)
			bundle.GetDirPath = "job-install-path"
		})

		ItInstallsJob := func(act func() error) {
			BeforeEach(func() {
				fs.TempDirDir = "/fake-tmp-dir"
			})

			It("returns error when installing job fails", func() {
				bundle.InstallError = errors.New("fake-install-error")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-install-error"))
			})

			It("downloads and later cleans up downloaded job template blob", func() {
				blobstore.GetReturns("/fake-blobstore-file-name", nil)

				err := act()
				Expect(err).ToNot(HaveOccurred())
				fingerPrint, signedURL, blobID, headers := blobstore.GetArgsForCall(0)
				Expect(blobID).To(Equal("fake-blobstore-id"))
				Expect(signedURL).To(Equal("/fake/signed/url"))
				Expect(headers).To(Equal(map[string]string{"key": "value"}))
				Expect(fingerPrint).To(Equal(boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-blob-sha1"))))

				// downloaded file is cleaned up
				_, cleanupArg := blobstore.CleanUpArgsForCall(0)
				Expect(cleanupArg).To(Equal("/fake-blobstore-file-name"))
			})

			It("returns error when downloading job template blob fails", func() {
				blobstore.GetReturns("", errors.New("fake-get-error"))

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-error"))
			})

			It("can process sha1 checksums in the new format", func() {
				blobstore.GetReturns("/fake-blobstore-file-name", nil)
				job.Source.Sha1 = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "sha1:fake-blob-sha1")

				err := act()
				Expect(err).ToNot(HaveOccurred())
				fingerPrint, signedURL, blobID, headers := blobstore.GetArgsForCall(0)
				Expect(blobID).To(Equal("fake-blobstore-id"))
				Expect(signedURL).To(Equal("/fake/signed/url"))
				Expect(headers).To(Equal(map[string]string{"key": "value"}))
				Expect(fingerPrint).To(Equal(boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "sha1:fake-blob-sha1"))))
			})

			It("can process sha2 checksums", func() {
				blobstore.GetReturns("/fake-blobstore-file-name", nil)
				job.Source.Sha1 = boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA256, "sha256:fake-blob-sha256")

				err := act()
				Expect(err).ToNot(HaveOccurred())
				fingerPrint, signedURL, blobID, headers := blobstore.GetArgsForCall(0)
				Expect(blobID).To(Equal("fake-blobstore-id"))
				Expect(signedURL).To(Equal("/fake/signed/url"))
				Expect(headers).To(Equal(map[string]string{"key": "value"}))
				Expect(fingerPrint).To(Equal(boshcrypto.MustNewMultipleDigest(job.Source.Sha1)))
			})

			It("installs bundle from decompressed tmp path of a job template", func() {
				blobstore.GetReturns("/fake-blobstore-file-name", nil)

				err := act()
				Expect(err).ToNot(HaveOccurred())

				// make sure that bundle install happened after decompression
				Expect(bundle.InstallSourcePath).To(Equal("/fake-blobstore-file-name"))
				Expect(bundle.InstallPathInBundle).To(Equal("fake-path-in-archive"))
			})

			It("fixes the permissions of the files in the job's install directory", func() {
				err := act()
				Expect(err).NotTo(HaveOccurred())
				Expect(fixPermissions.fakePathArg).To(Equal("job-install-path"))
				Expect(fixPermissions.fakeUserArg).To(Equal("root"))
				Expect(fixPermissions.fakeGroupArg).To(Equal("vcap"))
			})

			It("returns an errors when fixing permissions fails", func() {
				fixPermissions.fakeFixError = errors.New("disaster")

				err := act()
				Expect(err).To(HaveOccurred())
			})

			It("returns an errors when getting the install path fails", func() {
				bundle.GetDirError = errors.New("disaster")

				err := act()
				Expect(err).To(HaveOccurred())
			})
		}

		ItCreatesDirectories := func(act func() error) {
			It("creates work directories for a job", func() {
				err := act()

				Expect(err).ToNot(HaveOccurred())
				stat := fs.GetFileTestStat("/fakebasedir/data/sys/log/" + job.Name)
				Expect(stat).ToNot(BeNil())
				Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
				Expect(stat.Username).To(Equal("root"))
				Expect(stat.Groupname).To(Equal("vcap"))

				stat = fs.GetFileTestStat("/fakebasedir/data/sys/run/" + job.Name)
				Expect(stat).ToNot(BeNil())
				Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
				Expect(stat.Username).To(Equal("root"))
				Expect(stat.Groupname).To(Equal("vcap"))

				stat = fs.GetFileTestStat("/fakebasedir/data/" + job.Name)
				Expect(stat).ToNot(BeNil())
				Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
				Expect(stat.Username).To(Equal("root"))
				Expect(stat.Groupname).To(Equal("vcap"))
			})

			It("returns error when creating work directories fails", func() {
				fs.MkdirAllError = errors.New("fake-create-dirs-error")
				err := act()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Creating directories for job"))
			})
		}

		ItUpdatesPackages := func(act func() error) {
			var packageApplier *fakepackages.FakeApplier

			BeforeEach(func() {
				packageApplier = fakepackages.NewFakeApplier()
				packageApplierProvider.JobSpecificAppliers[job.Name] = packageApplier
			})

			It("applies each package that job depends on and then cleans up packages", func() {
				err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(packageApplier.ActionsCalled).To(Equal([]string{"Apply", "Apply", "KeepOnly"}))
				Expect(len(packageApplier.AppliedPackages)).To(Equal(2)) // present
				Expect(packageApplier.AppliedPackages).To(Equal(job.Packages))
			})

			It("returns error when applying package that job depends on fails", func() {
				packageApplier.ApplyError = errors.New("fake-apply-err")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-apply-err"))
			})

			It("keeps only currently required packages but does not completely uninstall them", func() {
				err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(packageApplier.KeptOnlyPackages)).To(Equal(2)) // present
				Expect(packageApplier.KeptOnlyPackages).To(Equal(job.Packages))
			})

			It("returns error when keeping only currently required packages fails", func() {
				packageApplier.KeepOnlyErr = errors.New("fake-keep-only-err")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-keep-only-err"))
			})
		}

		Describe("Prepare", func() {
			act := func() error {
				return applier.Prepare(job)
			}

			It("return an error if getting file bundle fails", func() {
				jobsBc.GetErr = errors.New("fake-get-bundle-error")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-bundle-error"))
			})

			It("returns an error if checking for installed path fails", func() {
				bundle.IsInstalledErr = errors.New("fake-is-installed-error")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-is-installed-error"))
			})

			Context("when job is already installed", func() {
				BeforeEach(func() {
					bundle.Installed = true
				})

				It("does not install", func() {
					err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(bundle.ActionsCalled).To(Equal([]string{})) // no Install
				})

				It("does not download the job template", func() {
					err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(blobstore.GetCallCount()).To(Equal(0))
				})
			})

			Context("when job is not installed", func() {
				BeforeEach(func() {
					bundle.Installed = false
				})

				It("installs job (but does not enable)", func() {
					err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(bundle.ActionsCalled).To(Equal([]string{"Install"}))
				})

				ItInstallsJob(act)
			})
		})

		Describe("Apply", func() {
			act := func() error {
				return applier.Apply(job)
			}

			It("return an error if getting file bundle fails", func() {
				jobsBc.GetErr = errors.New("fake-get-bundle-error")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-get-bundle-error"))
			})

			It("returns an error if checking for installed path fails", func() {
				bundle.IsInstalledErr = errors.New("fake-is-installed-error")

				err := act()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-is-installed-error"))
			})

			Context("when job is already installed", func() {
				BeforeEach(func() {
					bundle.Installed = true
				})

				It("does not install but only enables job", func() {
					err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(bundle.ActionsCalled).To(Equal([]string{"Enable"})) // no Install
				})

				It("returns error when job enable fails", func() {
					bundle.EnableError = errors.New("fake-enable-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-enable-error"))
				})

				It("does not download the job template", func() {
					err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(blobstore.GetCallCount()).To(Equal(0))
				})

				ItUpdatesPackages(act)
				ItCreatesDirectories(act)
			})

			Context("when job is not installed", func() {
				BeforeEach(func() {
					bundle.Installed = false
				})

				It("installs and enables job", func() {
					err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(bundle.ActionsCalled).To(Equal([]string{"Install", "Enable"}))
				})

				It("returns error when job enable fails", func() {
					bundle.EnableError = errors.New("fake-enable-error")

					err := act()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-enable-error"))
				})

				ItInstallsJob(act)

				ItUpdatesPackages(act)

				ItCreatesDirectories(act)
			})
		})
	})

	Describe("Configure", func() {
		It("adds job to the job supervisor", func() {
			job, bundle := buildJob(jobsBc)

			fs.WriteFileString("/path/to/job/monit", "some conf")
			fs.SetGlob("/path/to/job/*.monit", []string{"/path/to/job/subjob.monit"})

			bundle.GetDirPath = "/path/to/job"

			err := applier.Configure(job, 0)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(jobSupervisor.AddJobArgs)).To(Equal(2))

			Expect(jobSupervisor.AddJobArgs[0]).To(Equal(fakejobsuper.AddJobArgs{
				Name:       job.Name,
				Index:      0,
				ConfigPath: "/path/to/job/monit",
			}))

			Expect(jobSupervisor.AddJobArgs[1]).To(Equal(fakejobsuper.AddJobArgs{
				Name:       job.Name + "_subjob",
				Index:      0,
				ConfigPath: "/path/to/job/subjob.monit",
			}))
		})

		It("does not require monit script", func() {
			job, _ := buildJob(jobsBc)

			err := applier.Configure(job, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(jobSupervisor.AddJobArgs)).To(Equal(0))
		})
	})

	Describe("KeepOnly", func() {
		It("first disables and then uninstalls jobs that are not in keeponly list", func() {
			_, bundle1 := buildJob(jobsBc)
			job2, bundle2 := buildJob(jobsBc)
			_, bundle3 := buildJob(jobsBc)
			job4, bundle4 := buildJob(jobsBc)

			jobsBc.ListBundles = []boshbc.Bundle{bundle1, bundle2, bundle3, bundle4}

			err := applier.KeepOnly([]models.Job{job4, job2})
			Expect(err).ToNot(HaveOccurred())

			Expect(bundle1.ActionsCalled).To(Equal([]string{"Disable", "Uninstall"}))
			Expect(bundle2.ActionsCalled).To(Equal([]string{}))
			Expect(bundle3.ActionsCalled).To(Equal([]string{"Disable", "Uninstall"}))
			Expect(bundle4.ActionsCalled).To(Equal([]string{}))
		})

		It("returns error when bundle collection fails to return list of installed bundles", func() {
			jobsBc.ListErr = errors.New("fake-bc-list-error")

			err := applier.KeepOnly([]models.Job{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-bc-list-error"))
		})

		It("returns error when bundle collection cannot retrieve bundle for keep-only job", func() {
			job1, bundle1 := buildJob(jobsBc)

			jobsBc.ListBundles = []boshbc.Bundle{bundle1}
			jobsBc.GetErr = errors.New("fake-bc-get-error")

			err := applier.KeepOnly([]models.Job{job1})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-bc-get-error"))
		})

		It("returns error when at least one bundle cannot be disabled", func() {
			_, bundle1 := buildJob(jobsBc)

			jobsBc.ListBundles = []boshbc.Bundle{bundle1}
			bundle1.DisableErr = errors.New("fake-bc-disable-error")

			err := applier.KeepOnly([]models.Job{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-bc-disable-error"))
		})

		It("returns error when at least one bundle cannot be uninstalled", func() {
			_, bundle1 := buildJob(jobsBc)

			jobsBc.ListBundles = []boshbc.Bundle{bundle1}
			bundle1.UninstallErr = errors.New("fake-bc-uninstall-error")

			err := applier.KeepOnly([]models.Job{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-bc-uninstall-error"))
		})
	})

	Describe("DeleteSourceBlobs", func() {
		var jobOne, jobTwo, jobThree models.Job

		BeforeEach(func() {
			jobOne = models.Job{
				Source: models.Source{
					BlobstoreID: "blob-id",
				},
			}

			jobTwo = models.Job{
				Source: models.Source{
					BlobstoreID: "another-blob-id",
				},
			}

			jobThree = models.Job{
				Source: models.Source{
					BlobstoreID: "blob-id",
				},
			}
		})

		It("deletes the jobs source from the blobstore", func() {
			err := applier.DeleteSourceBlobs([]models.Job{jobOne, jobTwo, jobThree})
			Expect(err).NotTo(HaveOccurred())

			Expect(blobstore.DeleteCallCount()).To(Equal(2))
			_, deleteArg := blobstore.DeleteArgsForCall(0)
			Expect(deleteArg).To(Equal("blob-id"))
			_, deleteArg = blobstore.DeleteArgsForCall(1)
			Expect(deleteArg).To(Equal("another-blob-id"))
		})

		Context("when deleting from the blobstore fails", func() {
			BeforeEach(func() {
				blobstore.DeleteReturns(errors.New("something funny"))
			})

			It("returns the error", func() {
				err := applier.DeleteSourceBlobs([]models.Job{jobOne, jobTwo, jobThree})
				Expect(err).To(HaveOccurred())

				Expect(blobstore.DeleteCallCount()).To(Equal(1))
			})
		})
	})
})

type fakeFixer struct {
	fakeFixError error

	fakePathArg  string
	fakeUserArg  string
	fakeGroupArg string
}

func (f *fakeFixer) Fix(fs boshsys.FileSystem, path, user, group string) error {
	f.fakePathArg = path
	f.fakeUserArg = user
	f.fakeGroupArg = group

	return f.fakeFixError
}

type unsupportedAlgo struct{}

func (unsupportedAlgo) Compare(algo boshcrypto.Algorithm) int {
	return -1
}

func (unsupportedAlgo) CreateDigest(reader io.Reader) (boshcrypto.Digest, error) {
	return boshcrypto.MultipleDigest{}, nil
}

func buildJob(bc *fakebc.FakeBundleCollection) (models.Job, *fakebc.FakeBundle) {
	uuidGen := boshuuid.NewGenerator()
	uuid, err := uuidGen.Generate()
	Expect(err).ToNot(HaveOccurred())

	job := models.Job{
		Name:    "fake-job-name" + uuid,
		Version: "fake-job-version",
		Source: models.Source{
			Sha1:        boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-blob-sha1"),
			BlobstoreID: "fake-blobstore-id",
			SignedURL:   "/fake/signed/url",
			Headers: map[string]string{
				"key": "value",
			},
			PathInArchive: "fake-path-in-archive",
		},
		Packages: []models.Package{
			models.Package{
				Name:    "fake-package1-name" + uuid,
				Version: "fake-package1-version",
				Source: models.Source{
					Sha1:          boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-package1-sha1"),
					BlobstoreID:   "fake-package1-blobstore-id",
					PathInArchive: "",
				},
			},
			models.Package{
				Name:    "fake-package2-name" + uuid,
				Version: "fake-package2-version",
				Source: models.Source{
					Sha1:          boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-package2-sha1"),
					BlobstoreID:   "fake-package2-blobstore-id",
					PathInArchive: "",
				},
			},
		},
	}

	bundle := bc.FakeGet(job)

	return job, bundle
}
