package applier_test

import (
	"errors"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier"
	fakeas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	fakejobs "github.com/cloudfoundry/bosh-agent/v2/agent/applier/jobs/jobsfakes"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
	fakepackages "github.com/cloudfoundry/bosh-agent/v2/agent/applier/packages/fakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	"github.com/stretchr/testify/assert"
)

type FakeLogRotateDelegate struct {
	SetupLogrotateErr  error
	SetupLogrotateArgs SetupLogrotateArgs
}

type SetupLogrotateArgs struct {
	GroupName string
	BasePath  string
	Size      string
}

func (d *FakeLogRotateDelegate) SetupLogrotate(groupName, basePath, size string) error {
	d.SetupLogrotateArgs = SetupLogrotateArgs{groupName, basePath, size}
	return d.SetupLogrotateErr
}

func buildJob() models.Job {
	uuidGen := boshuuid.NewGenerator()
	uuid, err := uuidGen.Generate()
	Expect(err).ToNot(HaveOccurred())
	return models.Job{Name: "fake-job-name" + uuid, Version: "fake-version-name"}
}

func buildPackage() models.Package {
	uuidGen := boshuuid.NewGenerator()
	uuid, err := uuidGen.Generate()
	Expect(err).ToNot(HaveOccurred())
	return models.Package{Name: "fake-package-name" + uuid, Version: "fake-package-name"}
}

var _ = Describe("concreteApplier", func() {
	var (
		jobApplier        *fakejobs.FakeApplier
		packageApplier    *fakepackages.FakeApplier
		logRotateDelegate *FakeLogRotateDelegate
		jobSupervisor     *fakejobsuper.FakeJobSupervisor
		agentApplier      applier.Applier
		settingsService   boshsettings.Service
	)

	BeforeEach(func() {
		jobApplier = &fakejobs.FakeApplier{}
		packageApplier = fakepackages.NewFakeApplier()
		logRotateDelegate = &FakeLogRotateDelegate{}
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		settingsService = &fakesettings.FakeSettingsService{}
		agentApplier = applier.NewConcreteApplier(
			jobApplier,
			packageApplier,
			logRotateDelegate,
			jobSupervisor,
			boshdirs.NewProvider("/fake-base-dir"),
			settingsService.GetSettings(),
		)
	})

	Describe("Prepare", func() {
		It("prepares each jobs", func() {
			job := buildJob()

			err := agentApplier.Prepare(
				&fakeas.FakeApplySpec{JobResults: []models.Job{job}},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(jobApplier.PrepareCallCount()).To(Equal(1))
			Expect(jobApplier.PrepareArgsForCall(0)).To(Equal(job))
		})

		It("returns error when preparing jobs fails", func() {
			job := buildJob()

			jobApplier.PrepareReturns(errors.New("fake-prepare-job-error"))

			err := agentApplier.Prepare(
				&fakeas.FakeApplySpec{JobResults: []models.Job{job}},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-prepare-job-error"))
		})

		It("prepares each packages", func() {
			pkg1 := buildPackage()
			pkg2 := buildPackage()

			err := agentApplier.Prepare(
				&fakeas.FakeApplySpec{PackageResults: []models.Package{pkg1, pkg2}},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(packageApplier.PreparedPackages).To(ConsistOf(pkg1, pkg2))
		})

		It("returns error when preparing packages fails", func() {
			pkg := buildPackage()

			packageApplier.PrepareError = errors.New("fake-prepare-package-error")

			err := agentApplier.Prepare(
				&fakeas.FakeApplySpec{PackageResults: []models.Package{pkg}},
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-prepare-package-error"))
		})

		It("deletes the job source from the blobstore after preparing", func() {
			job := buildJob()

			err := agentApplier.Prepare(&fakeas.FakeApplySpec{JobResults: []models.Job{job}})
			Expect(err).ToNot(HaveOccurred())

			Expect(jobApplier.DeleteSourceBlobsCallCount()).To(Equal(1))
			Expect(jobApplier.DeleteSourceBlobsArgsForCall(0)).To(Equal([]models.Job{job}))
		})

		It("returns an error when deleting the source blobs fails", func() {
			job := buildJob()
			jobApplier.DeleteSourceBlobsReturns(errors.New("boom"))

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{job}})
			Expect(err).To(HaveOccurred())

			Expect(jobApplier.DeleteSourceBlobsCallCount()).To(Equal(1))
			Expect(jobApplier.DeleteSourceBlobsArgsForCall(0)).To(Equal([]models.Job{job}))
		})
	})

	Describe("Configure jobs", func() {
		It("reloads job supervisor", func() {
			job1 := models.Job{Name: "fake-job-name-1", Version: "fake-version-name-1"}
			job2 := models.Job{Name: "fake-job-name-2", Version: "fake-version-name-2"}
			jobs := []models.Job{job1, job2}

			err := agentApplier.ConfigureJobs(&fakeas.FakeApplySpec{JobResults: jobs})
			Expect(err).ToNot(HaveOccurred())

			Expect(jobSupervisor.Reloaded).To(BeTrue())
		})

		It("configures jobs in reverse order", func() {
			job1 := models.Job{Name: "fake-job-name-1", Version: "fake-version-name-1"}
			job2 := models.Job{Name: "fake-job-name-2", Version: "fake-version-name-2"}
			jobs := []models.Job{job1, job2}

			err := agentApplier.ConfigureJobs(&fakeas.FakeApplySpec{JobResults: jobs})
			Expect(err).ToNot(HaveOccurred())

			Expect(jobApplier.ConfigureCallCount()).To(Equal(2))
			job, _ := jobApplier.ConfigureArgsForCall(0)
			Expect(job).To(Equal(job2))
			job, _ = jobApplier.ConfigureArgsForCall(1)
			Expect(job).To(Equal(job1))
		})
	})

	Describe("Apply", func() {
		It("removes all jobs from job supervisor", func() {
			err := agentApplier.Apply(&fakeas.FakeApplySpec{})
			Expect(err).ToNot(HaveOccurred())

			Expect(jobSupervisor.RemovedAllJobs).To(BeTrue())
		})

		It("removes all previous jobs from job supervisor before starting to apply jobs", func() {
			// force remove all error
			jobSupervisor.RemovedAllJobsErr = errors.New("fake-remove-all-jobs-error")

			job := buildJob()
			agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{job}}) //nolint:errcheck

			// check that jobs were not applied before removing other jobs
			Expect(jobApplier.ApplyCallCount()).To(Equal(0))
		})

		It("returns error if removing all jobs from job supervisor fails", func() {
			jobSupervisor.RemovedAllJobsErr = errors.New("fake-remove-all-jobs-error")

			err := agentApplier.Apply(&fakeas.FakeApplySpec{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-remove-all-jobs-error"))
		})

		It("apply applies jobs", func() {
			job := buildJob()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{job}})

			Expect(err).ToNot(HaveOccurred())
			Expect(jobApplier.ApplyCallCount()).To(Equal(1))
			Expect(jobApplier.ApplyArgsForCall(0)).To(Equal(job))
		})

		It("apply errs when applying jobs errs", func() {
			job := buildJob()

			jobApplier.ApplyReturns(errors.New("fake-apply-job-error"))

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{job}})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-apply-job-error"))
		})

		It("asked jobApplier to keep only the jobs in the desired specs", func() {
			desiredJob := buildJob()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{desiredJob}})

			Expect(err).ToNot(HaveOccurred())

			Expect(jobApplier.KeepOnlyCallCount()).To(Equal(1))
			Expect(jobApplier.KeepOnlyArgsForCall(0)).To(Equal([]models.Job{desiredJob}))
		})

		It("returns error when jobApplier fails to keep only the jobs in the desired specs", func() {
			jobApplier.KeepOnlyReturns(errors.New("fake-keep-only-error"))

			desiredJob := buildJob()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{desiredJob}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-keep-only-error"))
		})

		It("apply applies packages", func() {
			pkg1 := buildPackage()
			pkg2 := buildPackage()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{PackageResults: []models.Package{pkg1, pkg2}})
			Expect(err).ToNot(HaveOccurred())
			Expect(packageApplier.AppliedPackages).To(Equal([]models.Package{pkg1, pkg2}))
		})

		It("apply errs when applying packages errs", func() {
			pkg := buildPackage()

			packageApplier.ApplyError = errors.New("fake-apply-package-error")

			err := agentApplier.Apply(&fakeas.FakeApplySpec{PackageResults: []models.Package{pkg}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-apply-package-error"))
		})

		It("asked packageApplier to keep only the packages in the desired specs", func() {
			desiredPkg := buildPackage()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{PackageResults: []models.Package{desiredPkg}})
			Expect(err).ToNot(HaveOccurred())
			Expect(packageApplier.KeptOnlyPackages).To(Equal([]models.Package{desiredPkg}))
		})

		It("returns error when packageApplier fails to keep only the packages in the desired specs", func() {
			packageApplier.KeepOnlyErr = errors.New("fake-keep-only-error")

			desiredPkg := buildPackage()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{PackageResults: []models.Package{desiredPkg}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-keep-only-error"))
		})

		It("apply does not configure jobs", func() {
			job1 := models.Job{Name: "fake-job-name-1", Version: "fake-version-name-1"}
			job2 := models.Job{Name: "fake-job-name-2", Version: "fake-version-name-2"}
			jobs := []models.Job{job1, job2}

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: jobs})
			Expect(err).ToNot(HaveOccurred())

			Expect(jobApplier.ConfigureCallCount()).To(Equal(0))

			Expect(jobSupervisor.Reloaded).To(BeTrue())
		})

		It("apply errs if monitor fails reload", func() {
			var jobs []models.Job
			jobSupervisor.ReloadErr = errors.New("error reloading monit")

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: jobs})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error reloading monit"))
		})

		It("apply sets up logrotation", func() {
			err := agentApplier.Apply(&fakeas.FakeApplySpec{MaxLogFileSizeResult: "fake-size"})
			Expect(err).ToNot(HaveOccurred())

			assert.Equal(GinkgoT(), logRotateDelegate.SetupLogrotateArgs, SetupLogrotateArgs{
				GroupName: boshsettings.VCAPUsername,
				BasePath:  filepath.Clean("/fake-base-dir"),
				Size:      "fake-size",
			})
		})

		It("apply errs if setup logrotate fails", func() {
			logRotateDelegate.SetupLogrotateErr = errors.New("fake-set-up-logrotate-error")

			err := agentApplier.Apply(&fakeas.FakeApplySpec{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-set-up-logrotate-error"))
		})

		It("deletes the job source from the blobstore after applying", func() {
			job := buildJob()

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{job}})
			Expect(err).ToNot(HaveOccurred())

			Expect(jobApplier.DeleteSourceBlobsCallCount()).To(Equal(1))
			Expect(jobApplier.DeleteSourceBlobsArgsForCall(0)).To(Equal([]models.Job{job}))
		})

		It("returns an error when deleting the source blobs fails", func() {
			job := buildJob()
			jobApplier.DeleteSourceBlobsReturns(errors.New("boom"))

			err := agentApplier.Apply(&fakeas.FakeApplySpec{JobResults: []models.Job{job}})
			Expect(err).To(HaveOccurred())

			Expect(jobApplier.DeleteSourceBlobsCallCount()).To(Equal(1))
			Expect(jobApplier.DeleteSourceBlobsArgsForCall(0)).To(Equal([]models.Job{job}))
		})
	})
})
