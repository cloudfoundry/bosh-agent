package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
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
