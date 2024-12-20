package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("SSHAction", func() {
	var (
		platform        *platformfakes.FakePlatform
		settingsService boshsettings.Service
		sshAction       action.SSHAction
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}

		platform = &platformfakes.FakePlatform{}
		dirProvider := boshdirs.NewProvider("/foo")
		logger := boshlog.NewLogger(boshlog.LevelNone)
		sshAction = action.NewSSH(settingsService, platform, dirProvider, logger)
	})

	AssertActionIsNotAsynchronous(sshAction)
	AssertActionIsNotPersistent(sshAction)
	AssertActionIsLoggable(sshAction)

	AssertActionIsNotResumable(sshAction)
	AssertActionIsNotCancelable(sshAction)

	Describe("Run", func() {
		Context("setupSSH", func() {
			var (
				response action.SSHResult
				params   action.SSHParams
				err      error

				defaultIP string

				platformPublicKeyValue string
				platformPublicKeyErr   error
			)

			BeforeEach(func() {
				defaultIP = "ww.xx.yy.zz"

				platformPublicKeyValue = ""
				platformPublicKeyErr = nil
			})

			JustBeforeEach(func() {
				settingsService := &fakesettings.FakeSettingsService{}
				settingsService.Settings.Networks = boshsettings.Networks{
					"fake-net": boshsettings.Network{IP: defaultIP},
				}

				platform.GetHostPublicKeyReturns(platformPublicKeyValue, platformPublicKeyErr)

				params = action.SSHParams{
					User:      "fake-user",
					PublicKey: "fake-public-key",
				}

				dirProvider := boshdirs.NewProvider("/foo")
				logger := boshlog.NewLogger(boshlog.LevelNone)
				sshAction = action.NewSSH(settingsService, platform, dirProvider, logger)
				response, err = sshAction.Run("setup", params)
			})

			Context("without default ip", func() {
				BeforeEach(func() {
					defaultIP = ""
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("No default ip"))
				})
			})

			Context("with an empty password", func() {
				It("should create user with an empty password", func() {
					Expect(err).ToNot(HaveOccurred())

					Expect(platform.CreateUserCallCount()).To(Equal(1))
					username, userPath := platform.CreateUserArgsForCall(0)
					Expect(username).To(Equal("fake-user"))
					Expect(userPath).To(boshassert.MatchPath("/foo/bosh_ssh"))

					Expect(platform.AddUserToGroupsCallCount()).To(Equal(1))
					user, groups := platform.AddUserToGroupsArgsForCall(0)
					Expect(user).To(Equal("fake-user"))
					Expect(groups).To(Equal(
						[]string{boshsettings.VCAPUsername, boshsettings.AdminGroup, boshsettings.SudoersGroup, boshsettings.SshersGroup},
					))

					Expect(platform.SetupSSHCallCount()).To(Equal(1))
					publicKeys, user := platform.SetupSSHArgsForCall(0)
					Expect(user).To(Equal("fake-user"))
					Expect(publicKeys).To(ConsistOf("fake-public-key"))
				})
			})

			Context("with a host public key available", func() {
				It("should return SSH Result with HostPublicKey", func() {
					hostPublicKey, _ := platform.GetHostPublicKey()
					Expect(response).To(Equal(action.SSHResult{
						Command:       "setup",
						Status:        "success",
						IP:            defaultIP,
						HostPublicKey: hostPublicKey,
					}))
					Expect(err).To(BeNil())
				})
			})

			Context("without a host public key available", func() {
				BeforeEach(func() {
					platformPublicKeyErr = errors.New("Get Host Public Key Failure")
				})

				It("should return an error", func() {
					Expect(response).To(Equal(action.SSHResult{}))
					Expect(err).ToNot(BeNil())
				})
			})
		})

		Context("cleanupSSH", func() {
			It("should delete ephemeral user", func() {
				response, err := sshAction.Run("cleanup", action.SSHParams{UserRegex: "^foobar.*"})
				Expect(err).ToNot(HaveOccurred())

				Expect(platform.DeleteEphemeralUsersMatchingCallCount()).To(Equal(1))
				Expect(platform.DeleteEphemeralUsersMatchingArgsForCall(0)).To(Equal("^foobar.*"))

				// Make sure empty ip field is not included in the response
				boshassert.MatchesJSONMap(GinkgoT(), response, map[string]interface{}{
					"command": "cleanup",
					"status":  "success",
				})
			})
		})
	})
})
