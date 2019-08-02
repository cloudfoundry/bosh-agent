package app

import (
	"fmt"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/clock"

	"os"

	boshagent "github.com/cloudfoundry/bosh-agent/agent"
	boshapplier "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshbc "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	boshaj "github.com/cloudfoundry/bosh-agent/agent/applier/jobs"
	boshap "github.com/cloudfoundry/bosh-agent/agent/applier/packages"
	boshagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore"
	boshrunner "github.com/cloudfoundry/bosh-agent/agent/cmdrunner"
	boshcomp "github.com/cloudfoundry/bosh-agent/agent/compiler"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
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
	var err error
	app.dirProvider = InitializeDirProvider(opts.BaseDirectory)
	app.logStemcellInfo()

	app.platform, err = NewPlatform(
		app,
		opts,
	)
	if err != nil {
		return bosherr.WrapError(err, "Getting platform")
	}

	boot, err := InitializeBootstrap(
		app,
		opts,
	)
	if err != nil {
		return bosherr.WrapError(err, "Initializing bootstrap")
	}

	if err = boot.Run(); err != nil {
		return bosherr.WrapError(err, "Running bootstrap")
	}

	app.agent, err = InitializeAgent(
		app,
		opts,
		time.Second*30,
	)
	if err != nil {
		return err
	}

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
