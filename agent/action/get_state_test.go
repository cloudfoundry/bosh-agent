package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	fakeas "github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/platform/vitals/vitalsfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	fakesettings "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
)

var _ = Describe("GetState", func() {
	var (
		settingsService *fakesettings.FakeSettingsService
		specService     *fakeas.FakeV1Service
		jobSupervisor   *fakejobsuper.FakeJobSupervisor
		vitalsService   *vitalsfakes.FakeService
		getStateAction  action.GetStateAction
	)

	BeforeEach(func() {
		settingsService = &fakesettings.FakeSettingsService{}
		jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
		specService = fakeas.NewFakeV1Service()
		vitalsService = &vitalsfakes.FakeService{}
		getStateAction = action.NewGetState(settingsService, specService, jobSupervisor, vitalsService)
	})

	AssertActionIsNotAsynchronous(getStateAction)
	AssertActionIsNotPersistent(getStateAction)
	AssertActionIsLoggable(getStateAction)

	AssertActionIsNotResumable(getStateAction)
	AssertActionIsNotCancelable(getStateAction)

	Describe("Run", func() {
		Context("when current spec can be retrieved", func() {
			Context("when vitals can be retrieved", func() {
				It("returns state", func() {
					settingsService.Settings.AgentID = "my-agent-id"
					settingsService.Settings.VM.Name = "vm-abc-def"

					jobSupervisor.StatusStatus = "running"

					specService.Spec = boshas.V1ApplySpec{
						Deployment: "fake-deployment",
					}

					expectedSpec := action.GetStateV1ApplySpec{
						V1ApplySpec: boshas.V1ApplySpec{
							NetworkSpecs:      map[string]boshas.NetworkSpec{},
							ResourcePoolSpecs: map[string]interface{}{},
							PackageSpecs:      map[string]boshas.PackageSpec{},
						},
						AgentID:  "my-agent-id",
						JobState: "running",
						VM:       boshsettings.VM{Name: "vm-abc-def"},
					}
					expectedSpec.Deployment = "fake-deployment"

					state, err := getStateAction.Run()
					Expect(err).ToNot(HaveOccurred())

					Expect(state.AgentID).To(Equal(expectedSpec.AgentID))
					Expect(state.JobState).To(Equal(expectedSpec.JobState))
					Expect(state.Deployment).To(Equal(expectedSpec.Deployment))
					boshassert.LacksJSONKey(GinkgoT(), state, "vitals")

					Expect(state).To(Equal(expectedSpec))
				})

				It("returns state in full format", func() {
					settingsService.Settings.AgentID = "my-agent-id"
					settingsService.Settings.VM.Name = "vm-abc-def"

					jobSupervisor.StatusStatus = "running"
					jobSupervisor.ProcessesStatus = []boshjobsuper.Process{
						boshjobsuper.Process{
							Name:  "fake-process-name-1",
							State: "running",
						},
						boshjobsuper.Process{
							Name:  "fake-process-name-2",
							State: "failing",
						},
					}

					specService.Spec = boshas.V1ApplySpec{
						Deployment: "fake-deployment",
					}

					expectedVitals := boshvitals.Vitals{
						Load: []string{"foo", "bar", "baz"},
					}

					vitalsService.GetReturns(expectedVitals, nil)
					expectedVM := map[string]interface{}{"name": "vm-abc-def"}

					expectedProcesses := []boshjobsuper.Process{
						boshjobsuper.Process{
							Name:  "fake-process-name-1",
							State: "running",
						},
						boshjobsuper.Process{
							Name:  "fake-process-name-2",
							State: "failing",
						},
					}

					state, err := getStateAction.Run("full")
					Expect(err).ToNot(HaveOccurred())

					boshassert.MatchesJSONString(GinkgoT(), state.AgentID, `"my-agent-id"`)
					boshassert.MatchesJSONString(GinkgoT(), state.JobState, `"running"`)
					boshassert.MatchesJSONString(GinkgoT(), state.Deployment, `"fake-deployment"`)
					Expect(*state.Vitals).To(Equal(expectedVitals))
					Expect(state.Processes).To(Equal(expectedProcesses))
					boshassert.MatchesJSONMap(GinkgoT(), state.VM, expectedVM)
				})

				Describe("non-populated field formatting", func() {
					It("returns network as empty hash if not set", func() {
						specService.Spec = boshas.V1ApplySpec{NetworkSpecs: nil}
						state, err := getStateAction.Run("full")
						Expect(err).ToNot(HaveOccurred())
						boshassert.MatchesJSONString(GinkgoT(), state.NetworkSpecs, `{}`)

						// Non-empty NetworkSpecs
						specService.Spec = boshas.V1ApplySpec{
							NetworkSpecs: map[string]boshas.NetworkSpec{
								"fake-net-name": boshas.NetworkSpec{
									Fields: map[string]interface{}{"ip": "fake-ip"},
								},
							},
						}
						state, err = getStateAction.Run("full")
						Expect(err).ToNot(HaveOccurred())
						boshassert.MatchesJSONString(GinkgoT(), state.NetworkSpecs, `{"fake-net-name":{"ip":"fake-ip"}}`)
					})

					It("returns resource_pool as empty hash if not set", func() {
						specService.Spec = boshas.V1ApplySpec{ResourcePoolSpecs: nil}
						state, err := getStateAction.Run("full")
						Expect(err).ToNot(HaveOccurred())
						boshassert.MatchesJSONString(GinkgoT(), state.ResourcePoolSpecs, `{}`)

						// Non-empty ResourcePoolSpecs
						specService.Spec = boshas.V1ApplySpec{ResourcePoolSpecs: "fake-resource-pool"}
						state, err = getStateAction.Run("full")
						Expect(err).ToNot(HaveOccurred())
						boshassert.MatchesJSONString(GinkgoT(), state.ResourcePoolSpecs, `"fake-resource-pool"`)
					})

					It("returns packages as empty hash if not set", func() {
						specService.Spec = boshas.V1ApplySpec{PackageSpecs: nil}
						state, err := getStateAction.Run("full")
						Expect(err).ToNot(HaveOccurred())
						boshassert.MatchesJSONString(GinkgoT(), state.PackageSpecs, `{}`)

						// Non-empty PackageSpecs
						specService.Spec = boshas.V1ApplySpec{PackageSpecs: map[string]boshas.PackageSpec{}}
						state, err = getStateAction.Run("full")
						Expect(err).ToNot(HaveOccurred())
						boshassert.MatchesJSONString(GinkgoT(), state.PackageSpecs, `{}`)
					})
				})
			})

			Context("when vitals cannot be retrieved", func() {
				BeforeEach(func() {
					vitalsService.GetReturns(boshvitals.Vitals{}, errors.New("fake-vitals-get-error"))
				})

				It("returns error", func() {
					_, err := getStateAction.Run("full")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake-vitals-get-error"))
				})
			})
		})

		Context("when current spec cannot be retrieved", func() {
			It("without current spec", func() {
				specService.GetErr = errors.New("fake-spec-get-error")

				_, err := getStateAction.Run()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-spec-get-error"))
			})
		})
	})
})
