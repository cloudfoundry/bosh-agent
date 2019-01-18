package bootonce_test

import (
	"errors"
	"path/filepath"

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

		startManager *bootonce.StartManager
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		var err error
		tmp, err = fs.TempDir("bootonce_agent")
		Expect(err).NotTo(HaveOccurred())

		settings = &fakesettings.FakeSettingsService{}
		dirProvider = boshdir.NewProvider(tmp)

		startManager = bootonce.NewStartManager(settings, fs, dirProvider)
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
				Expect(startManager.CanStart()).To(BeTrue())
			})
		})

		Context("when it is booting subsequent times", func() {
			BeforeEach(func() {
				startManager.RegisterStart()
			})

			It("allows the agent to boot", func() {
				Expect(startManager.CanStart()).To(BeTrue())
			})
		})
	})

	Context("when tmpfs is enabled", func() {
		tmpFsDisablesVMAgentReboot := func() {
			Context("when it is booting for the first time", func() {
				It("allows the agent to boot", func() {
					Expect(startManager.CanStart()).To(BeTrue())
				})
			})

			Context("when it is booting subsequent times after a VM reboot", func() {
				BeforeEach(func() {
					err := startManager.RegisterStart()
					Expect(err).NotTo(HaveOccurred())

					// delete the tmpfs bootonce to simulate a VM reboot
					err = fs.RemoveAll(filepath.Join(dirProvider.BoshSettingsDir(), bootonce.BootonceFileName))
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not allow the agent to boot", func() {
					Expect(startManager.CanStart()).To(BeFalse())
				})
			})

			Context("when the agent is restarting, but the VM didn't reboot", func() {
				BeforeEach(func() {
					err := startManager.RegisterStart()
					Expect(err).NotTo(HaveOccurred())
				})

				It("allows the agent to start", func() {
					Expect(startManager.CanStart()).To(BeTrue())
				})
			})

			Context("when the sentinel file fails to write", func() {
				BeforeEach(func() {
					fs.WriteFileError = errors.New("disaster")
				})

				It("RegisterStart returns an error", func() {
					err := startManager.RegisterStart()
					Expect(err).To(HaveOccurred())
				})
			})
		}

		Context("when the job_dir tmpfs feature flag is enabled", func() {
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

			tmpFsDisablesVMAgentReboot()
		})

		Context("when the settings dir tmpfs feature flag is enabled", func() {
			BeforeEach(func() {
				settings.Settings = boshsettings.Settings{
					Env: boshsettings.Env{
						Bosh: boshsettings.BoshEnv{
							Agent: boshsettings.AgentEnv{
								Settings: boshsettings.AgentSettings{
									TmpFS: true,
								},
							},
						},
					},
				}
			})

			tmpFsDisablesVMAgentReboot()
		})
	})
})
