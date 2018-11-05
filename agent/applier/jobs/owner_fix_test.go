package jobs

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("fixing job template permissions and ownership", func() {
	var fs *fakesys.FakeFileSystem

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()

		err := fs.MkdirAll("/jobs/bin", 0700)
		Expect(err).NotTo(HaveOccurred())

		err = fs.WriteFileString("/jobs/bin/run.sh", "i'm a binary 1001010101")
		Expect(err).NotTo(HaveOccurred())

		err = fs.WriteFileString("/jobs/binrun.sh", "i'm a binary 1001010101")
		Expect(err).NotTo(HaveOccurred())

		err = fs.MkdirAll("/jobs/config", 0700)
		Expect(err).NotTo(HaveOccurred())

		err = fs.WriteFileString("/jobs/config/file.ini", "i'm a config {}")
		Expect(err).NotTo(HaveOccurred())
	})

	It("makes the binary executable", func() {
		err := FixPermissions(fs, "/jobs", "root", "vcap")
		Expect(err).NotTo(HaveOccurred())

		runStat := fs.GetFileTestStat("/jobs/bin/run.sh")
		Expect(runStat.FileMode).To(Equal(os.FileMode(0750)))
		Expect(runStat.Username).To(Equal("root"))
		Expect(runStat.Groupname).To(Equal("vcap"))

		binStat := fs.GetFileTestStat("/jobs/bin")
		Expect(binStat.FileMode).To(Equal(os.FileMode(0750)))
		Expect(binStat.Username).To(Equal("root"))
		Expect(binStat.Groupname).To(Equal("vcap"))

		binRunStat := fs.GetFileTestStat("/jobs/binrun.sh")
		Expect(binRunStat.FileMode).To(Equal(os.FileMode(0640)))
		Expect(binRunStat.Username).To(Equal("root"))
		Expect(binRunStat.Groupname).To(Equal("vcap"))

		configDirStat := fs.GetFileTestStat("/jobs/config")
		Expect(configDirStat.FileMode).To(Equal(os.FileMode(0750)))
		Expect(configDirStat.Username).To(Equal("root"))
		Expect(configDirStat.Groupname).To(Equal("vcap"))

		configFileStat := fs.GetFileTestStat("/jobs/config/file.ini")
		Expect(configFileStat.FileMode).To(Equal(os.FileMode(0640)))
		Expect(configFileStat.Username).To(Equal("root"))
		Expect(configFileStat.Groupname).To(Equal("vcap"))
	})

	Context("when the walk fails", func() {
		It("errors", func() {
			fs.WalkErr = errors.New("disaster")
			err := FixPermissions(fs, "/jobs", "root", "vcap")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when chowning something fails", func() {
		It("errors", func() {
			fs.ChownErr = errors.New("disaster")
			err := FixPermissions(fs, "/jobs", "root", "vcap")
			Expect(err).To(HaveOccurred())
		})
	})
})
