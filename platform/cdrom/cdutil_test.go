package cdrom_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/cdrom"
	fakecdrom "github.com/cloudfoundry/bosh-agent/platform/cdrom/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("Cdutil", func() {
	var (
		fs     *fakesys.FakeFileSystem
		cd     *fakecdrom.FakeCdrom
		cdutil cdrom.CDUtil
		logger boshlog.Logger
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		cd = fakecdrom.NewFakeCdrom(fs, "env", "fake env contents")
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	JustBeforeEach(func() {
		cdutil = cdrom.NewCdUtil("/fake/settings/dir", fs, cd, logger)
	})

	It("gets file contents from CDROM", func() {
		contents, err := cdutil.GetFilesContents([]string{"env"})
		Expect(err).NotTo(HaveOccurred())

		Expect(cd.Mounted).To(Equal(false))
		Expect(cd.MediaAvailable).To(Equal(false))
		Expect(fs.FileExists("/fake/settings/dir")).To(Equal(true))
		Expect(cd.MountMountPath).To(Equal("/fake/settings/dir"))

		Expect(len(contents)).To(Equal(1))
		Expect(contents[0]).To(Equal([]byte("fake env contents")))
	})
})
