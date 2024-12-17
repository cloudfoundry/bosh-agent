package app

import (
	"fmt"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"

	"os"

	boshagent "github.com/cloudfoundry/bosh-agent/v2/agent"
	boshaction "github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshapplier "github.com/cloudfoundry/bosh-agent/v2/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	boshbc "github.com/cloudfoundry/bosh-agent/v2/agent/applier/bundlecollection"
	boshaj "github.com/cloudfoundry/bosh-agent/v2/agent/applier/jobs"
	boshap "github.com/cloudfoundry/bosh-agent/v2/agent/applier/packages"
	boshagentblobstore "github.com/cloudfoundry/bosh-agent/v2/agent/blobstore"
	"github.com/cloudfoundry/bosh-agent/v2/agent/bootonce"
	boshrunner "github.com/cloudfoundry/bosh-agent/v2/agent/cmdrunner"
	boshcomp "github.com/cloudfoundry/bosh-agent/v2/agent/compiler"
	httpblobprovider "github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider"
	"github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider/blobstore_delegator"
	boshscript "github.com/cloudfoundry/bosh-agent/v2/agent/script"
	boshtask "github.com/cloudfoundry/bosh-agent/v2/agent/task"
	boshinf "github.com/cloudfoundry/bosh-agent/v2/infrastructure"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	boshmonit "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/monit"
	boshmbus "github.com/cloudfoundry/bosh-agent/v2/mbus"
	boshnotif "github.com/cloudfoundry/bosh-agent/v2/notification"
	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	boshsigar "github.com/cloudfoundry/bosh-agent/v2/sigar"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	sigar "github.com/cloudfoundry/gosigar"
)

type App interface {
	Setup(opts Options) error
	Run() error
	GetPlatform() boshplatform.Platform
}

type app struct {
	logger      boshlog.Logger
	agent       boshagent.Agent
	platform    boshplatform.Platform
	fs          boshsys.FileSystem
	logTag      string
	dirProvider boshdirs.Provider
}

func New(logger boshlog.Logger, fs boshsys.FileSystem) App {
	return &app{
		logger: logger,
		fs:     fs,
		logTag: "App",
	}
}

func (app *app) Setup(opts Options) error {
	config, err := app.loadConfig(opts.ConfigPath)
	if err != nil {
		return bosherr.WrapError(err, "Loading config")
	}

	app.dirProvider = boshdirs.NewProvider(opts.BaseDirectory)
	app.logStemcellInfo()

	statsCollector := boshsigar.NewSigarStatsCollector(&sigar.ConcreteSigar{})
	auditLoggerProvider := boshplatform.NewAuditLoggerProvider()
	auditLogger := boshplatform.NewDelayedAuditLogger(auditLoggerProvider, app.logger)

	state, err := boshplatform.NewBootstrapState(app.fs, filepath.Join(app.dirProvider.BoshDir(), "agent_state.json"))
	if err != nil {
		return bosherr.WrapError(err, "Loading state")
	}

	timeService := clock.NewClock()
	platformProvider := boshplatform.NewProvider(app.logger, app.dirProvider, statsCollector, app.fs, config.Platform, state, timeService, auditLogger)

	app.platform, err = platformProvider.Get(opts.PlatformName)
	if err != nil {
		return bosherr.WrapError(err, "Getting platform")
	}

	settingsSourceFactory := boshinf.NewSettingsSourceFactory(config.Infrastructure.Settings, app.platform, app.logger)
	settingsSource, err := settingsSourceFactory.New()
	if err != nil {
		return bosherr.WrapError(err, "Getting Settings Source")
	}

	settingsService := boshsettings.NewService(
		app.platform.GetFs(),
		settingsSource,
		app.platform,
		app.logger,
	)

	specFilePath := filepath.Join(app.dirProvider.BoshDir(), "spec.json")
	specService := boshas.NewConcreteV1Service(
		app.platform.GetFs(),
		specFilePath,
	)

	boot := boshagent.NewBootstrap(
		app.platform,
		app.dirProvider,
		settingsService,
		specService,
		app.logger,
	)

	if err = boot.Run(); err != nil {
		return bosherr.WrapError(err, "Running bootstrap")
	}

	// For storing large non-sensitive blobs
	inconsiderateBlobManager, err := boshagentblobstore.NewBlobManager(app.dirProvider.BlobsDir())
	if err != nil {
		return bosherr.WrapError(err, "Getting blob manager")
	}

	// For storing sensitive blobs (rendered job templates)
	sensitiveBlobManager, err := boshagentblobstore.NewBlobManager(app.dirProvider.SensitiveBlobsDir())
	if err != nil {
		return bosherr.WrapError(err, "Getting blob manager")
	}

	blobstore, err := app.setupBlobstore(
		settingsService.GetSettings().GetBlobstore(),
		[]boshagentblobstore.BlobManagerInterface{sensitiveBlobManager, inconsiderateBlobManager},
	)
	if err != nil {
		return bosherr.WrapError(err, "Getting blobstore")
	}

	mbusHandlerProvider := boshmbus.NewHandlerProvider(settingsService, app.logger, auditLogger)

	mbusHandler, err := mbusHandlerProvider.Get(app.platform, inconsiderateBlobManager)
	if err != nil {
		return bosherr.WrapError(err, "Getting mbus handler")
	}

	monitClientProvider := boshmonit.NewProvider(app.platform, app.logger)

	monitClient, err := monitClientProvider.Get()
	if err != nil {
		return bosherr.WrapError(err, "Getting monit client")
	}

	jobSupervisorProvider := boshjobsuper.NewProvider(
		app.platform,
		monitClient,
		app.logger,
		app.dirProvider,
		mbusHandler,
	)

	jobSupervisor, err := jobSupervisorProvider.Get(opts.JobSupervisor)
	if err != nil {
		return bosherr.WrapError(err, "Getting job supervisor")
	}

	notifier := boshnotif.NewNotifier(mbusHandler)

	blobstoreHTTPClient, err := httpblobprovider.NewBlobstoreHTTPClient(settingsService.GetSettings().GetBlobstore())
	if err != nil {
		return bosherr.WrapError(err, "Failed constructing blobstore http client")
	}

	blobstoreDelegator := blobstore_delegator.NewBlobstoreDelegator(
		httpblobprovider.NewHTTPBlobImpl(app.platform.GetFs(), blobstoreHTTPClient),
		blobstore, app.logger,
	)

	applier, compiler := app.buildApplierAndCompiler(
		app.dirProvider,
		blobstoreDelegator,
		jobSupervisor,
		settingsService.GetSettings(),
		timeService,
	)

	uuidGen := boshuuid.NewGenerator()

	taskService := boshtask.NewAsyncTaskService(uuidGen, app.logger)

	taskManager := boshtask.NewManagerProvider().NewManager(
		app.logger,
		app.platform.GetFs(),
		app.dirProvider.BoshDir(),
	)

	jobScriptProvider := boshscript.NewConcreteJobScriptProvider(
		app.platform.GetRunner(),
		app.platform.GetFs(),
		app.platform.GetDirProvider(),
		timeService,
		app.logger,
	)

	actionFactory := boshaction.NewFactory(
		settingsService,
		app.platform,
		sensitiveBlobManager,
		taskService,
		notifier,
		applier,
		compiler,
		jobSupervisor,
		specService,
		jobScriptProvider,
		app.logger,
		blobstoreDelegator,
	)

	actionRunner := boshaction.NewRunner()

	actionDispatcher := boshagent.NewActionDispatcher(
		app.logger,
		taskService,
		taskManager,
		actionFactory,
		actionRunner,
	)

	startManager := bootonce.NewStartManager(
		settingsService,
		app.platform.GetFs(),
		app.dirProvider,
	)

	app.agent = boshagent.New(
		app.logger,
		mbusHandler,
		app.platform,
		actionDispatcher,
		jobSupervisor,
		specService,
		time.Second*30,
		settingsService,
		uuidGen,
		timeService,
		startManager,
	)

	return nil
}

func (app *app) Run() error {
	if err := app.agent.Run(); err != nil {
		return bosherr.WrapError(err, "Running agent")
	}
	return nil
}

func (app *app) GetPlatform() boshplatform.Platform {
	return app.platform
}

func (app *app) buildApplierAndCompiler(
	dirProvider boshdirs.Provider,
	blobstoreDelegator blobstore_delegator.BlobstoreDelegator,
	jobSupervisor boshjobsuper.JobSupervisor,
	settings boshsettings.Settings,
	timeService clock.Clock,
) (boshapplier.Applier, boshcomp.Compiler) {
	fileSystem := app.platform.GetFs()

	jobsBc := boshbc.NewFileBundleCollection(
		dirProvider.DataDir(),
		dirProvider.BaseDir(),
		"jobs",
		os.FileMode(0750),
		fileSystem,
		timeService,
		app.platform.GetCompressor(),
		app.logger,
	)

	packageApplierProvider := boshap.NewCompiledPackageApplierProvider(
		dirProvider.DataDir(),
		dirProvider.BaseDir(),
		dirProvider.JobsDir(),
		"packages",
		blobstoreDelegator,
		app.platform.GetCompressor(),
		fileSystem,
		timeService,
		app.logger,
	)

	jobApplier := boshaj.NewRenderedJobApplier(
		blobstoreDelegator,
		dirProvider,
		jobsBc,
		jobSupervisor,
		packageApplierProvider,
		boshaj.FixPermissions,
		fileSystem,
		app.logger,
	)

	applier := boshapplier.NewConcreteApplier(
		jobApplier,
		packageApplierProvider.Root(),
		app.platform,
		jobSupervisor,
		dirProvider,
		settings,
	)

	cmdRunner := boshrunner.NewFileLoggingCmdRunner(
		fileSystem,
		app.platform.GetRunner(),
		dirProvider.LogsDir(),
		10*1024, // 10 Kb
	)

	compiler := boshcomp.NewConcreteCompiler(
		app.platform.GetCompressor(),
		blobstoreDelegator,
		fileSystem,
		cmdRunner,
		dirProvider,
		packageApplierProvider.Root(),
		packageApplierProvider.RootBundleCollection(),
		clock.NewClock(),
	)

	return applier, compiler
}

func (app *app) loadConfig(path string) (Config, error) {
	// Use one off copy of file system to read configuration file
	fs := boshsys.NewOsFileSystem(app.logger)
	return LoadConfigFromPath(fs, path)
}

func (app *app) logStemcellInfo() {
	stemcellVersionFilePath := filepath.Join(app.dirProvider.EtcDir(), "stemcell_version")
	stemcellVersion := app.fileContents(stemcellVersionFilePath)
	stemcellSha1 := app.fileContents(filepath.Join(app.dirProvider.EtcDir(), "stemcell_git_sha1"))
	msg := fmt.Sprintf("Running on stemcell version '%s' (git: %s)", stemcellVersion, stemcellSha1)
	app.logger.Info(app.logTag, msg)
}

func (app *app) fileContents(path string) string {
	contents, err := app.fs.ReadFileString(path)
	if err != nil || len(contents) == 0 {
		contents = "?"
	}
	return contents
}

func (app *app) setupBlobstore(
	blobstoreSettings boshsettings.Blobstore,
	blobManagers []boshagentblobstore.BlobManagerInterface,
) (boshblob.DigestBlobstore, error) {
	blobstoreProvider := boshblob.NewProvider(
		app.platform.GetFs(),
		app.platform.GetRunner(),
		app.dirProvider.EtcDir(),
		app.logger,
	)

	blobstoreSettings = app.patchBlobstoreOptions(blobstoreSettings)

	blobstore, err := blobstoreProvider.Get(blobstoreSettings.Type, blobstoreSettings.Options)
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting blobstore")
	}

	return boshagentblobstore.NewCascadingBlobstore(blobstore, blobManagers, app.logger), nil
}

func (app *app) patchBlobstoreOptions(blobstoreSettings boshsettings.Blobstore) boshsettings.Blobstore {
	if blobstoreSettings.Type != boshblob.BlobstoreTypeLocal {
		return blobstoreSettings
	}

	blobstorePath, ok := blobstoreSettings.Options["blobstore_path"]
	if !ok {
		return blobstoreSettings
	}

	pathStr, ok := blobstorePath.(string)
	if !ok {
		return blobstoreSettings
	}

	if pathStr != "/var/vcap/micro_bosh/data/cache" {
		return blobstoreSettings
	}

	dir := app.dirProvider.BlobsDir()
	app.logger.Debug(app.logTag, fmt.Sprintf("Resetting local blobstore path to %s", dir))

	blobstoreSettings.Options = map[string]interface{}{
		"blobstore_path": dir,
	}

	return blobstoreSettings
}
