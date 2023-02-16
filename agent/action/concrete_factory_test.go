package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/script/scriptfakes"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshaction "github.com/cloudfoundry/bosh-agent/agent/action"
	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakeappl "github.com/cloudfoundry/bosh-agent/agent/applier/fakes"
	fakeagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore/blobstorefakes"
	fakecomp "github.com/cloudfoundry/bosh-agent/agent/compiler/fakes"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	faketask "github.com/cloudfoundry/bosh-agent/agent/task/fakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	fakenotif "github.com/cloudfoundry/bosh-agent/notification/fakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_clock.go code.cloudfoundry.org/clock.Clock

var _ = Describe("concreteFactory", func() {
	var (
		settingsService   *fakesettings.FakeSettingsService
		platform          *platformfakes.FakePlatform
		blobManager       *fakeagentblobstore.FakeBlobManagerInterface
		taskService       *faketask.FakeService
		notifier          *fakenotif.FakeNotifier
		applier           *fakeappl.FakeApplier
		compiler          *fakecomp.FakeCompiler
		jobSupervisor     *fakejobsuper.FakeJobSupervisor
		specService       *fakeas.FakeV1Service
		jobScriptProvider boshscript.JobScriptProvider
		factory           boshaction.Factory
		logger            boshlog.Logger
		fileSystem        *fakesys.FakeFileSystem
		blobDelegator     *fakeblobdelegator.FakeBlobstoreDelegator
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}

		platform = &platformfakes.FakePlatform{}
		fileSystem = fakesys.NewFakeFileSystem()
		platform.GetFsReturns(fileSystem)
		platform.GetDirProviderReturns(boshdir.NewProvider("/var/vcap"))

		blobManager = &fakeagentblobstore.FakeBlobManagerInterface{}
		taskService = &faketask.FakeService{}
		notifier = fakenotif.NewFakeNotifier()
		applier = fakeappl.NewFakeApplier()
		compiler = fakecomp.NewFakeCompiler()
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		specService = fakeas.NewFakeV1Service()
		jobScriptProvider = &scriptfakes.FakeJobScriptProvider{}
		logger = boshlog.NewLogger(boshlog.LevelNone)
		blobDelegator = &fakeblobdelegator.FakeBlobstoreDelegator{}

		factory = boshaction.NewFactory(
			settingsService,
			platform,
			blobManager,
			taskService,
			notifier,
			applier,
			compiler,
			jobSupervisor,
			specService,
			jobScriptProvider,
			logger,
			blobDelegator,
		)
	})

	It("returns error if boshaction cannot be created", func() {
		action, err := factory.Create("fake-unknown-boshaction")
		Expect(err).To(HaveOccurred())
		Expect(action).To(BeNil())
	})

	It("apply", func() {
		action, err := factory.Create("apply")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(BeEquivalentTo(boshaction.NewApply(
			applier,
			specService,
			settingsService,
			boshdir.NewProvider("/var/vcap"),
			fileSystem,
		)))
	})

	It("drain", func() {
		action, err := factory.Create("drain")
		Expect(err).ToNot(HaveOccurred())
		// Cannot do equality check since channel is used in initializer
		Expect(action).To(BeAssignableToTypeOf(boshaction.DrainAction{}))
	})

	It("fetch_logs", func() {
		action, err := factory.Create("fetch_logs")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewFetchLogs(platform.GetLogsTarProvider(), blobDelegator)))
	})

	It("fetch_logs_with_signed_url", func() {
		action, err := factory.Create("fetch_logs_with_signed_url")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewFetchLogsWithSignedURLAction(platform.GetLogsTarProvider(), blobDelegator)))
	})

	It("bundle_logs", func() {
		action, err := factory.Create("bundle_logs")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewBundleLogs(platform.GetLogsTarProvider(), platform.GetFs())))
	})

	It("remove_file", func() {
		action, err := factory.Create("remove_file")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewRemoveFile(platform.GetFs())))
	})

	It("get_task", func() {
		action, err := factory.Create("get_task")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewGetTask(taskService)))
	})

	It("cancel_task", func() {
		action, err := factory.Create("cancel_task")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewCancelTask(taskService)))
	})

	It("get_state", func() {
		action, err := factory.Create("get_state")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewGetState(settingsService, specService, jobSupervisor, platform.GetVitalsService())))
	})

	It("list_disk", func() {
		action, err := factory.Create("list_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewListDisk(settingsService, platform, logger)))
	})

	It("migrate_disk", func() {
		action, err := factory.Create("migrate_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewMigrateDisk(platform, platform.GetDirProvider())))
	})

	It("mount_disk", func() {
		action, err := factory.Create("mount_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewMountDisk(settingsService, platform, platform.GetDirProvider(), logger)))
	})

	It("ping", func() {
		action, err := factory.Create("ping")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewPing()))
	})

	It("info", func() {
		action, err := factory.Create("info")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewInfo()))
	})

	It("ssh", func() {
		action, err := factory.Create("ssh")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewSSH(settingsService, platform, platform.GetDirProvider(), logger)))
	})

	It("start", func() {
		action, err := factory.Create("start")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewStart(jobSupervisor, applier, specService)))
	})

	It("stop", func() {
		action, err := factory.Create("stop")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewStop(jobSupervisor)))
	})

	It("remove_persistent_disk", func() {
		action, err := factory.Create("remove_persistent_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewRemovePersistentDiskAction(settingsService)))
	})

	It("unmount_disk", func() {
		action, err := factory.Create("unmount_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewUnmountDisk(settingsService, platform)))
	})

	It("compile_package", func() {
		action, err := factory.Create("compile_package")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewCompilePackage(compiler)))
	})

	It("compile_package_with_signed_url", func() {
		action, err := factory.Create("compile_package_with_signed_url")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewCompilePackageWithSignedURL(compiler)))
	})

	It("run_errand", func() {
		action, err := factory.Create("run_errand")
		Expect(err).ToNot(HaveOccurred())

		// Cannot do equality check since channel is used in initializer
		Expect(action).To(BeAssignableToTypeOf(boshaction.RunErrandAction{}))
	})

	It("run_script", func() {
		action, err := factory.Create("run_script")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewRunScript(jobScriptProvider, specService, logger)))
	})

	It("prepare", func() {
		action, err := factory.Create("prepare")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewPrepare(applier)))
	})

	It("delete_arp_entries", func() {
		action, err := factory.Create("delete_arp_entries")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewDeleteARPEntries(platform)))
	})

	It("shutdown", func() {
		action, err := factory.Create("shutdown")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewShutdown(platform)))
	})

	It("sync_dns", func() {
		action, err := factory.Create("sync_dns")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(boshaction.NewSyncDNS(blobDelegator, settingsService, platform, logger)))
	})

	It("upload_blob", func() {
		action, err := factory.Create("upload_blob")
		Expect(err).ToNot(HaveOccurred())

		Expect(action).To(Equal(boshaction.NewUploadBlobAction(blobManager)))
	})
})
