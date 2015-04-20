package app

import (
	"path/filepath"
	"time"

	sigar "github.com/cloudfoundry/gosigar"

	boshagent "github.com/cloudfoundry/bosh-agent/agent"
	boshaction "github.com/cloudfoundry/bosh-agent/agent/action"
	boshapplier "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshbc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	boshaj "github.com/cloudfoundry/bosh-agent/agent/applier/jobs"
	boshap "github.com/cloudfoundry/bosh-agent/agent/applier/packages"
	boshrunner "github.com/cloudfoundry/bosh-agent/agent/cmdrunner"
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	boshdrain "github.com/cloudfoundry/bosh-agent/agent/drain"
	boshtask "github.com/cloudfoundry/bosh-agent/agent/task"
	boshblob "github.com/cloudfoundry/bosh-agent/blobstore"
	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshinf "github.com/cloudfoundry/bosh-agent/infrastructure"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshmonit "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshmbus "github.com/cloudfoundry/bosh-agent/mbus"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshsigar "github.com/cloudfoundry/bosh-agent/sigar"
	boshsyslog "github.com/cloudfoundry/bosh-agent/syslog"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
	boshtime "github.com/cloudfoundry/bosh-agent/time"
	boshuuid "github.com/cloudfoundry/bosh-agent/uuid"
)

type App interface {
	Setup(args []string) error
	Run() error
	GetPlatform() boshplatform.Platform
}

type app struct {
	logger   boshlog.Logger
	agent    boshagent.Agent
	platform boshplatform.Platform
}

func New(logger boshlog.Logger) App {
	return &app{logger: logger}
}

func (app *app) Setup(args []string) error {
	opts, err := ParseOptions(args)
	if err != nil {
		return bosherr.WrapError(err, "Parsing options")
	}

	config, err := app.loadConfig(opts.ConfigPath)
	if err != nil {
		return bosherr.WrapError(err, "Loading config")
	}

	dirProvider := boshdirs.NewProvider(opts.BaseDirectory)

	// Pulled outside of the platform provider so bosh-init will not pull in
	// sigar when cross compiling linux -> darwin
	sigarCollector := boshsigar.NewSigarStatsCollector(&sigar.ConcreteSigar{})

	platformProvider := boshplatform.NewProvider(app.logger, dirProvider, sigarCollector, config.Platform)
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
		filepath.Join(dirProvider.BoshDir(), "settings.json"),
		settingsSource,
		app.platform,
		app.logger,
	)
	boot := boshagent.NewBootstrap(
		app.platform,
		dirProvider,
		settingsService,
		app.logger,
	)

	if err = boot.Run(); err != nil {
		return bosherr.WrapError(err, "Running bootstrap")
	}

	mbusHandlerProvider := boshmbus.NewHandlerProvider(settingsService, app.logger)

	mbusHandler, err := mbusHandlerProvider.Get(app.platform, dirProvider)
	if err != nil {
		return bosherr.WrapError(err, "Getting mbus handler")
	}

	blobstoreProvider := boshblob.NewProvider(app.platform, dirProvider, app.logger)

	blobstore, err := blobstoreProvider.Get(settingsService.GetSettings().Blobstore)
	if err != nil {
		return bosherr.WrapError(err, "Getting blobstore")
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
		dirProvider,
		mbusHandler,
	)

	jobSupervisor, err := jobSupervisorProvider.Get(opts.JobSupervisor)
	if err != nil {
		return bosherr.WrapError(err, "Getting job supervisor")
	}

	notifier := boshnotif.NewNotifier(mbusHandler)

	applier, compiler := app.buildApplierAndCompiler(dirProvider, blobstore, jobSupervisor)

	uuidGen := boshuuid.NewGenerator()

	taskService := boshtask.NewAsyncTaskService(uuidGen, app.logger)

	taskManager := boshtask.NewManagerProvider().NewManager(
		app.logger,
		app.platform.GetFs(),
		dirProvider.BoshDir(),
	)

	specFilePath := filepath.Join(dirProvider.BoshDir(), "spec.json")
	specService := boshas.NewConcreteV1Service(
		app.platform.GetFs(),
		specFilePath,
	)

	drainScriptProvider := boshdrain.NewConcreteScriptProvider(
		app.platform.GetRunner(),
		app.platform.GetFs(),
		dirProvider,
	)

	actionFactory := boshaction.NewFactory(
		settingsService,
		app.platform,
		blobstore,
		taskService,
		notifier,
		applier,
		compiler,
		jobSupervisor,
		specService,
		drainScriptProvider,
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

	syslogServer := boshsyslog.NewServer(33331, app.logger)

	timeService := boshtime.NewConcreteService()

	app.agent = boshagent.New(
		app.logger,
		mbusHandler,
		app.platform,
		actionDispatcher,
		jobSupervisor,
		specService,
		syslogServer,
		time.Minute,
		settingsService,
		uuidGen,
		timeService,
	)

	return nil
}

func (app *app) Run() error {
	err := app.agent.Run()
	if err != nil {
		return bosherr.WrapError(err, "Running agent")
	}
	return nil
}

func (app *app) GetPlatform() boshplatform.Platform {
	return app.platform
}

func (app *app) buildApplierAndCompiler(
	dirProvider boshdirs.Provider,
	blobstore boshblob.Blobstore,
	jobSupervisor boshjobsuper.JobSupervisor,
) (boshapplier.Applier, boshcomp.Compiler) {
	jobsBc := boshbc.NewFileBundleCollection(
		dirProvider.DataDir(),
		dirProvider.BaseDir(),
		"jobs",
		app.platform.GetFs(),
		app.logger,
	)

	packageApplierProvider := boshap.NewCompiledPackageApplierProvider(
		dirProvider.DataDir(),
		dirProvider.BaseDir(),
		dirProvider.JobsDir(),
		"packages",
		blobstore,
		app.platform.GetCompressor(),
		app.platform.GetFs(),
		app.logger,
	)

	jobApplier := boshaj.NewRenderedJobApplier(
		jobsBc,
		jobSupervisor,
		packageApplierProvider,
		blobstore,
		app.platform.GetCompressor(),
		app.platform.GetFs(),
		app.logger,
	)

	applier := boshapplier.NewConcreteApplier(
		jobApplier,
		packageApplierProvider.Root(),
		app.platform,
		jobSupervisor,
		dirProvider,
	)

	platformRunner := app.platform.GetRunner()
	fileSystem := app.platform.GetFs()
	cmdRunner := boshrunner.NewFileLoggingCmdRunner(
		fileSystem,
		platformRunner,
		dirProvider.LogsDir(),
		10*1024, // 10 Kb
	)

	compiler := boshcomp.NewConcreteCompiler(
		app.platform.GetCompressor(),
		blobstore,
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
