package action

import (
	boshappl "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/drain"
	boshtask "github.com/cloudfoundry/bosh-agent/agent/task"
	boshblob "github.com/cloudfoundry/bosh-agent/blobstore"
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshntp "github.com/cloudfoundry/bosh-agent/platform/ntp"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type concreteFactory struct {
	availableActions map[string]Action
}

func NewFactory(
	settingsService boshsettings.Service,
	platform boshplatform.Platform,
	blobstore boshblob.Blobstore,
	taskService boshtask.Service,
	notifier boshnotif.Notifier,
	applier boshappl.Applier,
	compiler boshcomp.Compiler,
	jobSupervisor boshjobsuper.JobSupervisor,
	specService boshas.V1Service,
	drainScriptProvider boshdrain.DrainScriptProvider,
	logger boshlog.Logger,
) (factory Factory) {
	compressor := platform.GetCompressor()
	copier := platform.GetCopier()
	dirProvider := platform.GetDirProvider()
	vitalsService := platform.GetVitalsService()
	ntpService := boshntp.NewConcreteService(platform.GetFs(), dirProvider)

	factory = concreteFactory{
		availableActions: map[string]Action{
			// Task management
			"ping":        NewPing(),
			"get_task":    NewGetTask(taskService),
			"cancel_task": NewCancelTask(taskService),

			// VM admin
			"ssh":        NewSSH(settingsService, platform, dirProvider),
			"fetch_logs": NewFetchLogs(compressor, copier, blobstore, dirProvider),

			// Job management
			"prepare":    NewPrepare(applier),
			"apply":      NewApply(applier, specService, settingsService),
			"start":      NewStart(jobSupervisor),
			"stop":       NewStop(jobSupervisor),
			"drain":      NewDrain(notifier, specService, drainScriptProvider, jobSupervisor),
			"get_state":  NewGetState(settingsService, specService, jobSupervisor, vitalsService, ntpService),
			"run_errand": NewRunErrand(specService, dirProvider.JobsDir(), platform.GetRunner(), logger),

			// Compilation
			"compile_package":    NewCompilePackage(compiler),
			"release_apply_spec": NewReleaseApplySpec(platform),

			// Disk management
			"list_disk":    NewListDisk(settingsService, platform, logger),
			"migrate_disk": NewMigrateDisk(platform, dirProvider),
			"mount_disk":   NewMountDisk(settingsService, platform, platform, dirProvider),
			"unmount_disk": NewUnmountDisk(settingsService, platform),

			// Networking
			"prepare_network_change":     NewPrepareNetworkChange(platform.GetFs(), settingsService),
			"prepare_configure_networks": NewPrepareConfigureNetworks(platform, settingsService),
			"configure_networks":         NewConfigureNetworks(),
		},
	}
	return
}

func (f concreteFactory) Create(method string) (Action, error) {
	action, found := f.availableActions[method]
	if !found {
		return nil, bosherr.New("Could not create action with method %s", method)
	}

	return action, nil
}
