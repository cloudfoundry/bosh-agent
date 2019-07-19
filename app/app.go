package app

import (
	"fmt"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"

	"os"

	boshagent "github.com/cloudfoundry/bosh-agent/agent"
	boshaction "github.com/cloudfoundry/bosh-agent/agent/action"
	boshapplier "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshbc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	boshaj "github.com/cloudfoundry/bosh-agent/agent/applier/jobs"
	boshap "github.com/cloudfoundry/bosh-agent/agent/applier/packages"
	boshagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore"
	boshrunner "github.com/cloudfoundry/bosh-agent/agent/cmdrunner"
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshtask "github.com/cloudfoundry/bosh-agent/agent/task"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshmonit "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	boshmbus "github.com/cloudfoundry/bosh-agent/mbus"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
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

	app.dirProvider = InitializeDirProvider(opts.BaseDirectory)
	app.logStemcellInfo()

	auditLogger := InitializeAuditLogger(app.logger)

	state, err := boshplatform.NewBootstrapState(app.fs, filepath.Join(app.dirProvider.BoshDir(), "agent_state.json"))
	if err != nil {
		return bosherr.WrapError(err, "Loading state")
	}

	timeService := clock.NewClock()

	app.platform, err = NewPlatform(
		app.logger,
		app.dirProvider,
		app.fs,
		config.Platform,
		state,
		timeService,
		auditLogger,
		opts.PlatformName,
	)
	if err != nil {
		return bosherr.WrapError(err, "Getting platform")
	}

	settingsService, err := NewService(
		config.Infrastructure.Settings,
		app.platform,
		app.platform.GetFs(),
		app.platform,
		app.logger,
	)

	if err != nil {
		return bosherr.WrapError(err, "Getting Settings Source")
	}

	specService := NewSpecService(app.platform.GetFs(), app.dirProvider)

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

	logsBlobstore, err := settingsService.GetSettings().GetSpecificBlobstore("logs")
	if err != nil {
		return bosherr.WrapError(err, "Getting blobstore")
	}

	wrappedLogsBlobstore, err := app.setupBlobstore(
		logsBlobstore,
		[]boshagentblobstore.BlobManagerInterface{sensitiveBlobManager, inconsiderateBlobManager},
	)
	if err != nil {
		return bosherr.WrapError(err, "Getting blobstore")
	}

	packagesBlobstore, err := settingsService.GetSettings().GetSpecificBlobstore("packages")
	if err != nil {
		return bosherr.WrapError(err, "Getting blobstore")
	}

	wrappedPackagesBlobstore, err := app.setupBlobstore(
		packagesBlobstore,
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

	applier, compiler := app.buildApplierAndCompiler(
		app.dirProvider,
		wrappedPackagesBlobstore,
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
		wrappedPackagesBlobstore,
		wrappedLogsBlobstore,
		sensitiveBlobManager,
		taskService,
		notifier,
		applier,
		compiler,
		jobSupervisor,
		specService,
		jobScriptProvider,
		app.logger,
	)

	actionRunner := boshaction.NewRunner()

	actionDispatcher := boshagent.NewActionDispatcher(
		app.logger,
		taskService,
		taskManager,
		actionFactory,
		actionRunner,
	)

	app.agent = InitializeAgent(
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
		app.dirProvider,
		app.platform.GetFs(),
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
	packagesBlobstore boshblob.DigestBlobstore,
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
		packagesBlobstore,
		app.platform.GetCompressor(),
		fileSystem,
		timeService,
		app.logger,
	)

	jobApplier := boshaj.NewRenderedJobApplier(
		dirProvider,
		jobsBc,
		jobSupervisor,
		packageApplierProvider,
		packagesBlobstore,
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
		packagesBlobstore,
		fileSystem,
		cmdRunner,
		dirProvider,
		packageApplierProvider.Root(),
		packageApplierProvider.RootBundleCollection(),
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
	blobstoreConfigPath := filepath.Join(app.dirProvider.EtcDir(), blobstoreSettings.Name)
	if err := app.fs.MkdirAll(blobstoreConfigPath, 0755); err != nil {
		return nil, err
	}

	blobstoreProvider := boshblob.NewProvider(
		app.platform.GetFs(),
		app.platform.GetRunner(),
		blobstoreConfigPath,
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
