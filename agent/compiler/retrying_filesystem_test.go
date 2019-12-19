package compiler

import (
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)
func init() {
	Describe("Retrying file system", func() {
		It ("should call Stat once when Stat is called with same path", func() {
			normalFs := fakes.NewFakeFileSystem()
			fs := RetryingFs{normalFs}
			normalFs.WriteFile("mycoolfile", []byte{})
			fs.Stat("something")

			Expect(normalFs.StatCallCount).To(Equal(1))

		})

	})

}

