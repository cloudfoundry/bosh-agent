package action_test

import (
	"errors"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemoveFileAction", func() {
	var (
		fs *fakesys.FakeFileSystem

		action RemoveFileAction
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()

		action = NewRemoveFile(fs)
	})

	AssertActionIsLoggable(action)

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Describe("Run", func() {
		It("logs error if fs returns one", func() {
			fs.RemoveAllStub = func(path string) error {
				return errors.New("uh-oh")
			}

			_, err := action.Run("/tmp/foo")
			Expect(err).To(MatchError("uh-oh"))
		})

		It("invokes fs properly", func() {
			var arg string
			fs.RemoveAllStub = func(path string) error {
				arg = path
				return nil
			}

			_, err := action.Run("/tmp/foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(arg).To(Equal("/tmp/foo"))
		})

	})
})
