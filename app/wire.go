//+build wireinject

package app

import (
	"time"

	"code.cloudfoundry.org/clock"

	"github.com/cloudfoundry/bosh-agent/agent"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/agent/bootonce"
	boshhandler "github.com/cloudfoundry/bosh-agent/handler"
	"github.com/cloudfoundry/bosh-agent/infrastructure"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	"github.com/cloudfoundry/bosh-agent/platform"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	"github.com/cloudfoundry/bosh-agent/settings"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshsigar "github.com/cloudfoundry/bosh-agent/sigar"
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

func NewConcreteSigar() sigar.Sigar {
	return &sigar.ConcreteSigar{}
}

func NewPlatform(
	logger logger.Logger,
	dirProvider directories.Provider,
	fs system.FileSystem,
	opts platform.Options,
	state *platform.BootstrapState,
	clock clock.Clock,
	auditLogger platform.AuditLogger,
	name string,
) (boshplatform.Platform, error) {
	wire.Build(NewConcreteSigar, boshsigar.NewSigarStatsCollector, boshplatform.NewProvider, ProvidePlatform)

	return nil, nil
}

func ProvidePlatform(p boshplatform.Provider, name string) (boshplatform.Platform, error) {
	return p.Get(name)
}

func InitializeSettingsSourceFactory(opts infrastructure.SettingsOptions, platform platform.Platform, logger logger.Logger) infrastructure.SettingsSourceFactory {
	wire.Build(infrastructure.NewSettingsSourceFactory)

	return infrastructure.SettingsSourceFactory{}
}

func NewStartManager(settingsService settings.Service, fs boshsys.FileSystem, dirProvider boshdirs.Provider) agent.StartManager {
	return bootonce.NewStartManager(
		settingsService,
		fs,
		dirProvider,
	)
}

func InitializeAgent(
	logger boshlog.Logger,
	mbusHandler boshhandler.Handler,
	platform boshplatform.Platform,
	actionDispatcher agent.ActionDispatcher,
	jobSupervisor boshjobsuper.JobSupervisor,
	specService boshas.V1Service,
	heartbeatInterval time.Duration,
	settingsService boshsettings.Service,
	uuidGenerator boshuuid.Generator,
	timeService clock.Clock,
	dirProvider boshdirs.Provider,
	fs boshsys.FileSystem,
) agent.Agent {

	wire.Build(NewStartManager, agent.New)
	return agent.Agent{}
}
