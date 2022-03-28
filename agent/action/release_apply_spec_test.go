package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("ReleaseApplySpec", func() {
	var (
		platform               *platformfakes.FakePlatform
		releaseApplySpecAction action.ReleaseApplySpecAction
		fileSystem             *fakesys.FakeFileSystem
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		fileSystem = fakesys.NewFakeFileSystem()
		platform.GetFsReturns(fileSystem)
		releaseApplySpecAction = action.NewReleaseApplySpec(platform)
	})

	AssertActionIsNotAsynchronous(releaseApplySpecAction)
	AssertActionIsNotPersistent(releaseApplySpecAction)
	AssertActionIsLoggable(releaseApplySpecAction)

	AssertActionIsNotResumable(releaseApplySpecAction)
	AssertActionIsNotCancelable(releaseApplySpecAction)

	It("run", func() {
		err := fileSystem.WriteFileString("/var/vcap/micro/apply_spec.json", `{"json":["objects"]}`)
		Expect(err).ToNot(HaveOccurred())

		value, err := releaseApplySpecAction.Run()
		Expect(err).ToNot(HaveOccurred())

		Expect(value).To(Equal(map[string]interface{}{"json": []interface{}{"objects"}}))
	})
})
