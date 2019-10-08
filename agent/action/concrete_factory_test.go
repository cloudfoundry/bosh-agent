package action_test

import (
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/script/scriptfakes"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	boshscript "github.com/cloudfoundry/bosh-agent/agent/script"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakeappl "github.com/cloudfoundry/bosh-agent/agent/applier/fakes"
	fakeagentblobstore "github.com/cloudfoundry/bosh-agent/agent/blobstore/blobstorefakes"
	fakecomp "github.com/cloudfoundry/bosh-agent/agent/compiler/fakes"
	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	faketask "github.com/cloudfoundry/bosh-agent/agent/task/fakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	fakenotif "github.com/cloudfoundry/bosh-agent/notification/fakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

//go:generate counterfeiter -o fakes/fake_clock.go ../../vendor/code.cloudfoundry.org/clock Clock

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
		factory           Factory
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

		factory = NewFactory(
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

	It("returns error if action cannot be created", func() {
		action, err := factory.Create("fake-unknown-action")
		Expect(err).To(HaveOccurred())
		Expect(action).To(BeNil())
	})

	It("apply", func() {
		action, err := factory.Create("apply")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(BeEquivalentTo(NewApply(
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
		Expect(action).To(BeAssignableToTypeOf(DrainAction{}))
	})

	It("fetch_logs", func() {
		action, err := factory.Create("fetch_logs")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewFetchLogs(platform.GetCompressor(), platform.GetCopier(), blobDelegator, platform.GetDirProvider())))
	})

	It("fetch_logs_with_signed_url", func() {
		ac, err := factory.Create("fetch_logs_with_signed_url")
		Expect(err).ToNot(HaveOccurred())

		Expect(ac).To(Equal(NewFetchLogsWithSignedURLAction(platform.GetCompressor(), platform.GetCopier(), platform.GetDirProvider(), blobDelegator)))
	})

	It("get_task", func() {
		action, err := factory.Create("get_task")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewGetTask(taskService)))
	})

	It("cancel_task", func() {
		action, err := factory.Create("cancel_task")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewCancelTask(taskService)))
	})

	It("get_state", func() {
		action, err := factory.Create("get_state")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewGetState(settingsService, specService, jobSupervisor, platform.GetVitalsService())))
	})

	It("list_disk", func() {
		action, err := factory.Create("list_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewListDisk(settingsService, platform, logger)))
	})

	It("migrate_disk", func() {
		action, err := factory.Create("migrate_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewMigrateDisk(platform, platform.GetDirProvider())))
	})

	It("mount_disk", func() {
		action, err := factory.Create("mount_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewMountDisk(settingsService, platform, platform.GetDirProvider(), logger)))
	})

	It("ping", func() {
		action, err := factory.Create("ping")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewPing()))
	})

	It("info", func() {
		action, err := factory.Create("info")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewInfo()))
	})

	It("ssh", func() {
		action, err := factory.Create("ssh")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewSSH(settingsService, platform, platform.GetDirProvider(), logger)))
	})

	It("start", func() {
		action, err := factory.Create("start")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewStart(jobSupervisor, applier, specService)))
	})

	It("stop", func() {
		action, err := factory.Create("stop")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewStop(jobSupervisor)))
	})

	It("remove_persistent_disk", func() {
		action, err := factory.Create("remove_persistent_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewRemovePersistentDiskAction(settingsService)))
	})

	It("unmount_disk", func() {
		action, err := factory.Create("unmount_disk")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewUnmountDisk(settingsService, platform)))
	})

	It("compile_package", func() {
		action, err := factory.Create("compile_package")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewCompilePackage(compiler)))
	})

	It("compile_package_with_signed_url", func() {
		action, err := factory.Create("compile_package_with_signed_url")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewCompilePackageWithSignedURL(compiler)))
	})

	It("run_errand", func() {
		action, err := factory.Create("run_errand")
		Expect(err).ToNot(HaveOccurred())

		// Cannot do equality check since channel is used in initializer
		Expect(action).To(BeAssignableToTypeOf(RunErrandAction{}))
	})

	It("run_script", func() {
		action, err := factory.Create("run_script")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewRunScript(jobScriptProvider, specService, logger)))
	})

	It("prepare", func() {
		action, err := factory.Create("prepare")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewPrepare(applier)))
	})

	It("delete_arp_entries", func() {
		action, err := factory.Create("delete_arp_entries")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewDeleteARPEntries(platform)))
	})

	It("shutdown", func() {
		action, err := factory.Create("shutdown")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewShutdown(platform)))
	})

	It("sync_dns", func() {
		action, err := factory.Create("sync_dns")
		Expect(err).ToNot(HaveOccurred())
		Expect(action).To(Equal(NewSyncDNS(blobDelegator, settingsService, platform, logger)))
	})

	It("upload_blob", func() {
		action, err := factory.Create("upload_blob")
		Expect(err).ToNot(HaveOccurred())

		Expect(action).To(Equal(NewUploadBlobAction(blobManager)))
	})
})
