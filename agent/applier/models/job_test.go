package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/applier/models"
	"github.com/cloudfoundry/bosh-agent/settings/directories"
	"github.com/cloudfoundry/bosh-utils/crypto"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	"os"

	"errors"
)

var _ = Describe("Job", func() {
	var job Job
	BeforeEach(func() {
		job = Job{
			Name: "fake-name",
		}
	})

	Describe("BundleName", func() {
		It("returns name", func() {
			Expect(job.BundleName()).To(Equal("fake-name"))
		})
	})

	Describe("BundleVersion", func() {
		BeforeEach(func() {
			job = Job{
				Version: "fake-version",
				Source:  Source{Sha1: crypto.NewDigest(crypto.DigestAlgorithmSHA1, "fake-sha1")},
			}
		})

		It("returns version plus sha1 of source to make jobs unique", func() {
			Expect(job.BundleVersion()).To(Equal("fake-version-fake-sha1"))
		})
	})

	Describe("CreateDirectories", func() {
		var (
			fs          *fakesys.FakeFileSystem
			dirProvider directories.Provider
		)

		BeforeEach(func() {
			fs = fakes.NewFakeFileSystem()
			dirProvider = directories.NewProvider("/fakebasedir")
		})

		Context("when directory already exists", func() {
			BeforeEach(func() {
				err := fs.MkdirAll("/fakebasedir/data/sys/log/"+job.Name, 0600)
				Expect(err).ToNot(HaveOccurred())
				err = fs.Chown("/fakebasedir/data/sys/log/"+job.Name, "maximus:maximus")
				Expect(err).ToNot(HaveOccurred())
				fs.MkdirAllCallCount = 0
				fs.ChownCallCount = 0
			})

			It("should not MkdirAll, chmod, chown for existing directories", func() {
				err := job.CreateDirectories(fs, dirProvider)
				Expect(err).ToNot(HaveOccurred())

				Expect(fs.MkdirAllCallCount).To(Equal(2))
				Expect(fs.ChmodCallCount).To(Equal(2))
				Expect(fs.ChownCallCount).To(Equal(2))

				stat := fs.GetFileTestStat("/fakebasedir/data/sys/log/" + job.Name)
				Expect(stat).ToNot(BeNil())
				Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(stat.FileMode).To(Equal(os.FileMode(0600)))
				Expect(stat.Username).To(Equal("maximus"))
				Expect(stat.Groupname).To(Equal("maximus"))

				stat = fs.GetFileTestStat("/fakebasedir/data/" + job.Name)
				Expect(stat).ToNot(BeNil())
				Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
				Expect(stat.Username).To(Equal("root"))
				Expect(stat.Groupname).To(Equal("vcap"))

				stat = fs.GetFileTestStat("/fakebasedir/data/sys/run/" + job.Name)
				Expect(stat).ToNot(BeNil())
				Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
				Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
				Expect(stat.Username).To(Equal("root"))
				Expect(stat.Groupname).To(Equal("vcap"))
			})
		})

		It("creates the jobs directories", func() {
			err := job.CreateDirectories(fs, dirProvider)
			Expect(err).ToNot(HaveOccurred())

			stat := fs.GetFileTestStat("/fakebasedir/data/sys/log/" + job.Name)
			Expect(stat).ToNot(BeNil())
			Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
			Expect(stat.Username).To(Equal("root"))
			Expect(stat.Groupname).To(Equal("vcap"))

			stat = fs.GetFileTestStat("/fakebasedir/data/sys/run/" + job.Name)
			Expect(stat).ToNot(BeNil())
			Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
			Expect(stat.Username).To(Equal("root"))
			Expect(stat.Groupname).To(Equal("vcap"))

			stat = fs.GetFileTestStat("/fakebasedir/data/" + job.Name)
			Expect(stat).ToNot(BeNil())
			Expect(stat.FileType).To(Equal(fakesys.FakeFileTypeDir))
			Expect(stat.FileMode).To(Equal(os.FileMode(0770)))
			Expect(stat.Username).To(Equal("root"))
			Expect(stat.Groupname).To(Equal("vcap"))
		})

		Context("when the filesystem fails to create a dir", func() {
			BeforeEach(func() {
				fs.MkdirAllError = errors.New("fs-is-busted")
			})

			It("promotes the error", func() {
				err := job.CreateDirectories(fs, dirProvider)
				Expect(err).To(HaveOccurred())

				stat := fs.GetFileTestStat("/fakebasedir/data/sys/log/" + job.Name)
				Expect(stat).To(BeNil())
			})
		})

		Context("when the filesystem fails to chown a dir", func() {
			BeforeEach(func() {
				fs.ChownErr = errors.New("fs-is-busted")
			})

			It("promotes the error", func() {
				err := job.CreateDirectories(fs, dirProvider)
				Expect(err).To(HaveOccurred())

				stat := fs.GetFileTestStat("/fakebasedir/data/sys/log/" + job.Name)
				Expect(stat).ToNot(BeNil())
			})
		})

		Context("when an invalid jobname is provided", func() {
			BeforeEach(func() {
				job.Name = ""
			})

			It("should return an error", func() {
				err := job.CreateDirectories(fs, dirProvider)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
