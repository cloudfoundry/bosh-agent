//+build wireinject

package app

import (
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"

	"github.com/cloudfoundry/bosh-agent/agent"
	boshagent "github.com/cloudfoundry/bosh-agent/agent"
	boshaction "github.com/cloudfoundry/bosh-agent/agent/action"
	boshapplier "github.com/cloudfoundry/bosh-agent/agent/applier"
	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/agent/blobstore"
	boshagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore"
	"github.com/cloudfoundry/bosh-agent/agent/bootonce"
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshtask "github.com/cloudfoundry/bosh-agent/agent/task"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	"github.com/cloudfoundry/bosh-agent/infrastructure"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshmonit "github.com/cloudfoundry/bosh-agent/jobsupervisor/monit"
	boshmbus "github.com/cloudfoundry/bosh-agent/mbus"
	boshnotif "github.com/cloudfoundry/bosh-agent/notification"
	"github.com/cloudfoundry/bosh-agent/platform"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	"github.com/cloudfoundry/bosh-agent/settings"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshsigar "github.com/cloudfoundry/bosh-agent/sigar"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
	sigar "github.com/cloudfoundry/gosigar"
	"github.com/google/wire"
)

func InitializeDirProvider(baseDir string) directories.Provider {
	wire.Build(boshdirs.NewProvider)

	return directories.Provider{}
}

func InitializeAuditLogger(logger boshlog.Logger) *platform.DelayedAuditLogger {
	wire.Build(boshplatform.NewAuditLoggerProvider, boshplatform.NewDelayedAuditLogger)

	return nil
}

func NewConcreteSigar() *sigar.ConcreteSigar {
	return &sigar.ConcreteSigar{}
}

func NewPlatform(
	a *app,
	o Options,
) (boshplatform.Platform, error) {
	wire.Build(
		NewConcreteSigar,
		wire.Bind(new(sigar.Sigar), new(*sigar.ConcreteSigar)),
		boshsigar.NewSigarStatsCollector,
		boshplatform.NewProvider,
		clock.NewClock,
		InitializeAuditLogger,
		wire.Bind(new(platform.AuditLogger), new(*platform.DelayedAuditLogger)),
		ProvidePlatform,
		ProvideAppLogger,
		ProvideAppDirProvider,
		ProvideAppFileSystem,
		ProvidePlatformOptions,
		ProvidePlatformName,
		ProvideConfig,
		InitializeBootstrapState,
	)

	return nil, nil
}

func InitializeBootstrap(a *app, o Options) (agent.Bootstrap, error) {
	wire.Build(
		NewSettingsSourceFactory,
		ProvideSettings,
		ProvideConfig,
		ProvideSettingsSource,
		ProvideAppPlatform,
		ProvideAppDirProvider,
		ProvidePlatformFS,
		ProvideAppLogger,
		NewSpecService,
		boshagent.NewBootstrap,
		ProvideAppPlatformSettingsGetter,
		settings.NewService,
	)
	return nil, nil
}

func ProvideBlobsDir(p directories.Provider) string {
	return p.BlobsDir()
}

func ProvideConfig(a *app, o Options) (Config, error) {
	return a.loadConfig(o.ConfigPath)
}

func InitializeBootstrapState(
	a *app,
) (*platform.BootstrapState, error) {
	wire.Build(
		ProvideAppFileSystem,
		ProvideAgentStateFilePath,
		boshplatform.NewBootstrapState,
	)

	return nil, nil
}

func InitializeNotifier(a *app, o Options) (boshnotif.Notifier, error) {
	wire.Build(
		NewSettingsSourceFactory,
		ProvideSettingsSource,
		ProvideSettings,
		ProvideConfig,
		ProvideAppDirProvider,
		ProvideInconsiderateBlobsDir,
		ProvideAppLogger,
		InitializeAuditLogger,
		wire.Bind(new(platform.AuditLogger), new(*platform.DelayedAuditLogger)),
		ProvideAppPlatform,
		ProvideAppPlatformSettingsGetter,
		ProvidePlatformFS,
		settings.NewService,
		boshmbus.NewHandlerProvider,
		ProvideMbusHandler,
		InitializeBlobManager,
		wire.Bind(new(blobstore.BlobManagerInterface), new(*boshagentblobstore.BlobManager)),
		boshnotif.NewNotifier,
	)
	return nil, nil
}

func ProvideLogsBlobstore(a *app, s settings.Settings) (boshagentblobstore.LogsBlobstore, error) {
	wrappedLogsBlobstore, err := ProvideWrappedBlobstore(a, s, "logs")
	if err != nil {
		return nil, err
	}

	return boshagentblobstore.LogsBlobstore(wrappedLogsBlobstore), nil
}

func ProvidePackagesBlobstore(a *app, s settings.Settings) (boshagentblobstore.PackagesBlobstore, error) {
	wrappedPackagesBlobstore, err := ProvideWrappedBlobstore(a, s, "packages")
	if err != nil {
		return nil, err
	}

	return boshagentblobstore.PackagesBlobstore(wrappedPackagesBlobstore), nil
}

func InitializeBlobManager(a *app, blobsDir string) (*boshagentblobstore.BlobManager, error) {
	wire.Build(
		boshagentblobstore.NewBlobManager,
	)
	return nil, nil
}

func ProvideJobSupervisor(p boshjobsuper.Provider, o Options) (boshjobsuper.JobSupervisor, error) {
	return p.Get(o.JobSupervisor)
}

func InitializeJobSupervisorProvider(
	ssf infrastructure.SettingsSourceFactory,
	bss boshsettings.Source,
	pp platform.Platform,
	mc boshmonit.Client,
	inconsiderate_blobs_dir string,
	l logger.Logger,
	p boshmbus.HandlerProvider,
	bmi blobstore.BlobManagerInterface,
	a *app,
	o Options,
) (boshjobsuper.Provider, error) {
	wire.Build(
		boshjobsuper.NewProvider,
		ProvideAppDirProvider,
		ProvideMbusHandler,
	)
	return boshjobsuper.Provider{}, nil
}

func ProvideInconsiderateBlobsDir(dp directories.Provider) string {
	return dp.BlobsDir()
}

func ProvideSensitiveBlobsDir(dp directories.Provider) string {
	return dp.SensitiveBlobsDir()
}

func ProvideMonitClient(p boshmonit.ClientProvider) (boshmonit.Client, error) {
	return p.Get()
}

func ProvideMbusHandler(p boshmbus.HandlerProvider, ap platform.Platform, b boshagentblobstore.BlobManagerInterface) (boshhandler.Handler, error) {
	return p.Get(ap, b)
}

func ProvideWrappedBlobstore(a *app, s settings.Settings, blobstorePurpose string) (boshblob.DigestBlobstore, error) {
	b, err := s.GetSpecificBlobstore(blobstorePurpose)
	if err != nil {
		return nil, err
	}
	// For storing large non-sensitive blobs
	inconsiderateBlobManager, err := InitializeBlobManager(a, a.dirProvider.BlobsDir())
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting blob manager")
	}

	// For storing sensitive blobs (rendered job templates)
	sensitiveBlobManager, err := InitializeBlobManager(a, a.dirProvider.SensitiveBlobsDir())
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting blob manager")
	}

	return a.setupBlobstore(
		b,
		[]boshagentblobstore.BlobManagerInterface{sensitiveBlobManager, inconsiderateBlobManager},
	)
}

func ProvideServiceSettings(s settings.Service) settings.Settings {
	return s.GetSettings()
}

func ProvideAgentStateFilePath(a *app) string {
	return filepath.Join(a.dirProvider.BoshDir(), "agent_state.json")
}

func ProvidePlatformOptions(c Config) platform.Options {
	return c.Platform
}

func ProvidePlatformName(o Options) string {
	return o.PlatformName
}

func ProvideSettings(c Config) infrastructure.SettingsOptions {
	return c.Infrastructure.Settings
}

func ProvidePlatform(p boshplatform.Provider, name string) (boshplatform.Platform, error) {
	return p.Get(name)
}

func ProvideSettingsSource(ssf infrastructure.SettingsSourceFactory) (boshsettings.Source, error) {
	return ssf.New()
}

func NewSettingsSourceFactory(opts infrastructure.SettingsOptions, platform platform.Platform, logger logger.Logger) infrastructure.SettingsSourceFactory {
	wire.Build(infrastructure.NewSettingsSourceFactory)

	return infrastructure.SettingsSourceFactory{}
}

func ProvideSpecFilePath(dp directories.Provider) string {
	return filepath.Join(dp.BoshDir(), "spec.json")
}

func ProvideAppFileSystem(a *app) system.FileSystem {
	return a.fs
}

func ProvideAppPlatform(a *app) platform.Platform {
	return a.platform
}

func ProvideAppDirProvider(a *app) directories.Provider {
	return a.dirProvider
}

func ProvideAppPlatformSettingsGetter(a *app) boshsettings.PlatformSettingsGetter {
	return a.platform
}

func ProvidePlatformFS(p platform.Platform) boshsys.FileSystem {
	return p.GetFs()
}

func ProvideAppLogger(a *app) logger.Logger {
	return a.logger
}

func NewSpecService(
	fs system.FileSystem,
	d directories.Provider,
) applyspec.V1Service {
	wire.Build(
		boshas.NewConcreteV1Service,
		ProvideSpecFilePath,
	)

	return nil
}

func ProvideTaskManager(
	t boshtask.ManagerProvider,
	l logger.Logger,
	f system.FileSystem,
	dir string,
) boshtask.Manager {
	return t.NewManager(l, f, dir)
}

func ProvideApplier(
	a *app,
	b boshagentblobstore.PackagesBlobstore,
	j boshjobsuper.JobSupervisor,
	s boshsettings.Settings,
	t clock.Clock,
) boshapplier.Applier {
	applier, _ := a.buildApplierAndCompiler(
		a.dirProvider,
		b,
		j,
		s,
		t,
	)
	return applier
}

func ProvideCompiler(
	a *app,
	b boshagentblobstore.PackagesBlobstore,
	j boshjobsuper.JobSupervisor,
	s boshsettings.Settings,
	t clock.Clock,
) boshcomp.Compiler {
	_, compiler := a.buildApplierAndCompiler(
		a.dirProvider,
		b,
		j,
		s,
		t,
	)

	return compiler
}

func ProvideCmdRunner(p platform.Platform) system.CmdRunner {
	return p.GetRunner()
}

func InitializeAgent(
	a *app,
	o Options,
	heartbeatInterval time.Duration,
) (agent.Agent, error) {

	wire.Build(
		bootonce.NewStartManager,
		wire.Bind(new(agent.StartManager), new(*bootonce.StartManager)),
		InitializeAuditLogger,
		ProvideAppLogger,
		ProvideAppPlatform,
		boshmonit.NewProvider,
		ProvideMonitClient,
		NewSettingsSourceFactory,
		ProvideSettings,
		ProvideSettingsSource,
		ProvideConfig,
		boshagent.NewActionDispatcher,
		boshtask.NewAsyncTaskService,
		boshtask.NewManagerProvider,
		ProvideTaskManager,
		ProvideCmdRunner,
		clock.NewClock,
		boshaction.NewRunner,
		ProvidePackagesBlobstore,
		ProvideLogsBlobstore,
		ProvideServiceSettings,
		ProvideApplier,
		ProvideCompiler,
		boshscript.NewConcreteJobScriptProvider,
		wire.Bind(new(boshscript.JobScriptProvider), new(boshscript.ConcreteJobScriptProvider)),
		InitializeNotifier,
		boshaction.NewFactory,
		boshuuid.NewGenerator,
		InitializeJobSupervisorProvider,
		ProvideAppPlatformSettingsGetter,
		settings.NewService,
		ProvideAppDirProvider,
		ProvideJobSupervisor,
		NewSpecService,
		ProvidePlatformFS,
		wire.Bind(new(platform.AuditLogger), new(*platform.DelayedAuditLogger)),
		InitializeBlobManager,
		ProvideInconsiderateBlobsDir,
		wire.Bind(new(blobstore.BlobManagerInterface), new(*boshagentblobstore.BlobManager)),
		boshmbus.NewHandlerProvider,
		ProvideMbusHandler,
		agent.New,
	)
	return agent.Agent{}, nil
}
