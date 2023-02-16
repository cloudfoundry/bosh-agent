package action

import (
	boshappl "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshagentblob "github.com/cloudfoundry/bosh-agent/agent/blobstore"
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	blobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshtask "github.com/cloudfoundry/bosh-agent/agent/task"
	"github.com/cloudfoundry/bosh-agent/agent/utils"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type concreteFactory struct {
	availableActions map[string]Action
}

func NewFactory(
	settingsService boshsettings.Service,
	platform boshplatform.Platform,
	// TODO(ctz, ja): refactor the usage of blobstore as its a duplicate to the
	// last argument.
	sensitiveBlobManager boshagentblob.BlobManagerInterface,
	taskService boshtask.Service,
	notifier boshnotif.Notifier,
	applier boshappl.Applier,
	compiler boshcomp.Compiler,
	jobSupervisor boshjobsuper.JobSupervisor,
	specService boshas.V1Service,
	jobScriptProvider boshscript.JobScriptProvider,
	logger boshlog.Logger,
	blobstoreDelegator blobdelegator.BlobstoreDelegator) (factory Factory) {
	dirProvider := platform.GetDirProvider()
	vitalsService := platform.GetVitalsService()
	certManager := platform.GetCertManager()
	logsTarProvider := platform.GetLogsTarProvider()

	return concreteFactory{
		availableActions: map[string]Action{
			// API
			"ping": NewPing(),
			"info": NewInfo(),

			// Task management
			"get_task":    NewGetTask(taskService),
			"cancel_task": NewCancelTask(taskService),

			// VM admin
			"ssh":                        NewSSH(settingsService, platform, dirProvider, logger),
			"bundle_logs":                NewBundleLogs(logsTarProvider, platform.GetFs()),
			"fetch_logs":                 NewFetchLogs(logsTarProvider, blobstoreDelegator),
			"fetch_logs_with_signed_url": NewFetchLogsWithSignedURLAction(logsTarProvider, blobstoreDelegator),
			"update_settings":            NewUpdateSettings(settingsService, platform, certManager, logger, utils.NewAgentKiller()),
			"shutdown":                   NewShutdown(platform),
			"remove_file":                NewRemoveFile(platform.GetFs()),

			// Job management
			"prepare":    NewPrepare(applier),
			"apply":      NewApply(applier, specService, settingsService, dirProvider, platform.GetFs()),
			"start":      NewStart(jobSupervisor, applier, specService),
			"stop":       NewStop(jobSupervisor),
			"drain":      NewDrain(notifier, specService, jobScriptProvider, jobSupervisor, logger),
			"get_state":  NewGetState(settingsService, specService, jobSupervisor, vitalsService),
			"run_errand": NewRunErrand(specService, dirProvider.JobsDir(), platform.GetRunner(), logger),
			"run_script": NewRunScript(jobScriptProvider, specService, logger),

			// Compilation
			"compile_package":                 NewCompilePackage(compiler),
			"compile_package_with_signed_url": NewCompilePackageWithSignedURL(compiler),

			// Rendered Templates
			"upload_blob": NewUploadBlobAction(sensitiveBlobManager),

			// Disk management
			"list_disk":              NewListDisk(settingsService, platform, logger),
			"migrate_disk":           NewMigrateDisk(platform, dirProvider),
			"mount_disk":             NewMountDisk(settingsService, platform, dirProvider, logger),
			"unmount_disk":           NewUnmountDisk(settingsService, platform),
			"add_persistent_disk":    NewAddPersistentDiskAction(settingsService),
			"remove_persistent_disk": NewRemovePersistentDiskAction(settingsService),

			// ARP cache management
			"delete_arp_entries": NewDeleteARPEntries(platform),

			// DNS
			"sync_dns":                 NewSyncDNS(blobstoreDelegator, settingsService, platform, logger),
			"sync_dns_with_signed_url": NewSyncDNSWithSignedURL(settingsService, platform, logger, blobstoreDelegator),
		},
	}
}

func (f concreteFactory) Create(method string) (Action, error) {
	action, found := f.availableActions[method]
	if !found {
		return nil, bosherr.Errorf("Could not create action with method %s", method)
	}

	return action, nil
}
