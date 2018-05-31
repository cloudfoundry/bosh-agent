package action_test

import (
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("ReleaseApplySpec", func() {
	var (
		platform   *platformfakes.FakePlatform
		action     ReleaseApplySpecAction
		fileSystem *fakesys.FakeFileSystem
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		fileSystem = fakesys.NewFakeFileSystem()
		platform.GetFsReturns(fileSystem)
		action = NewReleaseApplySpec(platform)
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	It("run", func() {
		err := fileSystem.WriteFileString("/var/vcap/micro/apply_spec.json", `{"json":["objects"]}`)
		Expect(err).ToNot(HaveOccurred())

		value, err := action.Run()
		Expect(err).ToNot(HaveOccurred())

		Expect(value).To(Equal(map[string]interface{}{"json": []interface{}{"objects"}}))
	})
})
