package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshassert "github.com/cloudfoundry/bosh-utils/assert"
)

var _ = Describe("Testing with Ginkgo", func() {
	var (
		migrateDiskAction action.MigrateDiskAction
		platform          *platformfakes.FakePlatform
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		dirProvider := boshdirs.NewProvider("/foo")
		migrateDiskAction = action.NewMigrateDisk(platform, dirProvider)
	})

	AssertActionIsAsynchronous(migrateDiskAction)
	AssertActionIsNotPersistent(migrateDiskAction)
	AssertActionIsLoggable(migrateDiskAction)

	AssertActionIsNotResumable(migrateDiskAction)
	AssertActionIsNotCancelable(migrateDiskAction)

	It("migrate disk migrateDiskAction run", func() {
		value, err := migrateDiskAction.Run()
		Expect(err).ToNot(HaveOccurred())
		boshassert.MatchesJSONString(GinkgoT(), value, "{}")

		Expect(platform.MigratePersistentDiskCallCount()).To(Equal(1))
		fromPath, toPath := platform.MigratePersistentDiskArgsForCall(0)
		Expect(fromPath).To(boshassert.MatchPath("/foo/store"))
		Expect(toPath).To(boshassert.MatchPath("/foo/store_migration_target"))
	})
})
