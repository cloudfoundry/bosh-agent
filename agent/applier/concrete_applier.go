package applier

import (
	as "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/jobs"
	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/packages"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/work"
)

type concreteApplier struct {
	jobApplier        jobs.Applier
	packageApplier    packages.Applier
	logrotateDelegate LogrotateDelegate
	jobSupervisor     boshjobsuper.JobSupervisor
	dirProvider       boshdirs.Provider
	settings          boshsettings.Settings
}

func NewConcreteApplier(
	jobApplier jobs.Applier,
	packageApplier packages.Applier,
	logrotateDelegate LogrotateDelegate,
	jobSupervisor boshjobsuper.JobSupervisor,
	dirProvider boshdirs.Provider,
	settings boshsettings.Settings,
) Applier {
	return &concreteApplier{
		jobApplier:        jobApplier,
		packageApplier:    packageApplier,
		logrotateDelegate: logrotateDelegate,
		jobSupervisor:     jobSupervisor,
		dirProvider:       dirProvider,
		settings:          settings,
	}
}

func (a *concreteApplier) Prepare(desiredApplySpec as.ApplySpec) error {
	tasks := make([]func() error, 0, len(desiredApplySpec.Jobs())+len(desiredApplySpec.Packages()))

	pool := work.Pool{
		Count: *a.settings.Env.GetParallel(),
	}

	for _, job := range desiredApplySpec.Jobs() {
		job := job
		tasks = append(tasks, func() error {
			jobErr := a.jobApplier.Prepare(job)
			if jobErr != nil {
				return bosherr.WrapErrorf(jobErr, "Preparing job %s", job.Name)
			}
			return nil
		})
	}

	for _, pkg := range desiredApplySpec.Packages() {
		pkg := pkg
		tasks = append(tasks, func() error {
			pkgErr := a.packageApplier.Prepare(pkg)
			if pkgErr != nil {
				return bosherr.WrapErrorf(pkgErr, "Preparing package %s", pkg.Name)
			}
			return nil
		})
	}

	err := pool.ParallelDo(tasks...)
	if err != nil {
		return err
	}

	err = a.jobApplier.DeleteSourceBlobs(desiredApplySpec.Jobs())
	if err != nil {
		return bosherr.WrapError(err, "Failed removing job source blobs")
	}

	return nil
}

func (a *concreteApplier) Apply(desiredApplySpec as.ApplySpec) error {
	err := a.jobSupervisor.RemoveAllJobs()
	if err != nil {
		return bosherr.WrapError(err, "Removing all jobs")
	}

	jobs := desiredApplySpec.Jobs()
	for _, job := range jobs {
		err = a.jobApplier.Apply(job)
		if err != nil {
			return bosherr.WrapErrorf(err, "Applying job %s", job.Name)
		}
	}

	err = a.jobApplier.DeleteSourceBlobs(desiredApplySpec.Jobs())
	if err != nil {
		return bosherr.WrapError(err, "Failed removing job source blobs")
	}

	err = a.jobApplier.KeepOnly(desiredApplySpec.Jobs())
	if err != nil {
		return bosherr.WrapError(err, "Keeping only needed jobs")
	}

	for _, pkg := range desiredApplySpec.Packages() {
		err = a.packageApplier.Apply(pkg)
		if err != nil {
			return bosherr.WrapErrorf(err, "Applying package %s", pkg.Name)
		}
	}

	err = a.packageApplier.KeepOnly(desiredApplySpec.Packages())
	if err != nil {
		return bosherr.WrapError(err, "Keeping only needed packages")
	}

	err = a.jobSupervisor.Reload()
	if err != nil {
		return bosherr.WrapError(err, "Reloading jobSupervisor")
	}

	return a.setUpLogrotate(desiredApplySpec)
}

func (a *concreteApplier) ConfigureJobs(desiredApplySpec as.ApplySpec) error {
	jobs := desiredApplySpec.Jobs()
	for i := 0; i < len(jobs); i++ {
		job := jobs[len(jobs)-1-i]

		err := a.jobApplier.Configure(job, i)
		if err != nil {
			return bosherr.WrapErrorf(err, "Configuring job %s", job.Name)
		}
	}

	err := a.jobSupervisor.Reload()
	if err != nil {
		return bosherr.WrapError(err, "Reloading jobSupervisor")
	}

	return nil
}

func (a *concreteApplier) setUpLogrotate(applySpec as.ApplySpec) error {
	err := a.logrotateDelegate.SetupLogrotate(
		boshsettings.VCAPUsername,
		a.dirProvider.BaseDir(),
		applySpec.MaxLogFileSize(),
	)
	if err != nil {
		return bosherr.WrapError(err, "Logrotate setup failed")
	}

	return nil
}
