package bootonce_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	"github.com/cloudfoundry/bosh-agent/agent/bootonce"
)

var _ = Describe("checking if the agent can be booted", func() {
	var (
		tmp string

		fs          *fakesys.FakeFileSystem
		settings    *fakesettings.FakeSettingsService
		dirProvider boshdir.Provider

		rebootChecker *bootonce.RebootChecker
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		var err error
		tmp, err = fs.TempDir("bootonce_agent")
		Expect(err).NotTo(HaveOccurred())

		settings = &fakesettings.FakeSettingsService{}
		dirProvider = boshdir.NewProvider(tmp)

		rebootChecker = bootonce.NewRebootChecker(settings, fs, dirProvider)
	})

	Context("when the tmpfs feature flag is disabled", func() {
		BeforeEach(func() {
			settings.Settings = boshsettings.Settings{
				Env: boshsettings.Env{
					Bosh: boshsettings.BoshEnv{
						JobDir: boshsettings.JobDir{
							TmpFs: false,
						},
					},
				},
			}
		})

		Context("when it is booting for the first time", func() {
			It("allows the agent to boot", func() {
				bootable, err := rebootChecker.CanReboot()
				Expect(err).NotTo(HaveOccurred())

				Expect(bootable).To(BeTrue())
			})
		})

		Context("when it is booting subsequent times", func() {
			BeforeEach(func() {
				_, err := rebootChecker.CanReboot()
				Expect(err).NotTo(HaveOccurred())
			})

			It("allows the agent to boot", func() {
				bootable, err := rebootChecker.CanReboot()
				Expect(err).NotTo(HaveOccurred())

				Expect(bootable).To(BeTrue())
			})
		})
	})

	Context("when the tmpfs feature flag is enabled", func() {
		BeforeEach(func() {
			settings.Settings = boshsettings.Settings{
				Env: boshsettings.Env{
					Bosh: boshsettings.BoshEnv{
						JobDir: boshsettings.JobDir{
							TmpFs: true,
						},
					},
				},
			}
		})

		Context("when it is booting for the first time", func() {
			It("allows the agent to boot", func() {
				bootable, err := rebootChecker.CanReboot()
				Expect(err).NotTo(HaveOccurred())

				Expect(bootable).To(BeTrue())
			})
		})

		Context("when it is booting subsequent times", func() {
			BeforeEach(func() {
				_, err := rebootChecker.CanReboot()
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not allow the agent to boot", func() {
				bootable, err := rebootChecker.CanReboot()
				Expect(err).NotTo(HaveOccurred())

				Expect(bootable).To(BeFalse())
			})
		})

		Context("when the sentinel file fails to write", func() {
			BeforeEach(func() {
				fs.WriteFileError = errors.New("disaster")
			})

			It("returns an error", func() {
				_, err := rebootChecker.CanReboot()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
