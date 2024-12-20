package directories_test

import (
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/v2/settings/directories"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Provider", func() {
	p := directories.NewProvider(filepath.FromSlash("/some/dir"))
	DescribeTable("the directory paths",
		func(actual string, expected string) {
			Expect(actual).To(Equal(filepath.FromSlash(expected)))
		},
		Entry("BaseDir()", p.BaseDir(), "/some/dir"),
		Entry("BoshDir()", p.BoshDir(), "/some/dir/bosh"),
		Entry("BoshSettingsDir()", p.BoshSettingsDir(), "/some/dir/bosh/settings"),
		Entry("BoshBinDir()", p.BoshBinDir(), "/some/dir/bosh/bin"),
		Entry("EtcDir()", p.EtcDir(), "/some/dir/bosh/etc"),
		Entry("StoreDir()", p.StoreDir(), "/some/dir/store"),
		Entry("DataDir()", p.DataDir(), "/some/dir/data"),
		Entry("StoreMigrationDir()", p.StoreMigrationDir(), "/some/dir/store_migration_target"),
		Entry("PkgDir()", p.PkgDir(), "/some/dir/data/packages"),
		Entry("CompileDir()", p.CompileDir(), "/some/dir/data/compile"),
		Entry("MonitJobsDir()", p.MonitJobsDir(), "/some/dir/monit/job"),
		Entry("MonitDir()", p.MonitDir(), "/some/dir/monit"),
		Entry("JobsDir()", p.JobsDir(), "/some/dir/jobs"),
		Entry("DataJobsDir()", p.DataJobsDir(), "/some/dir/data/jobs"),
		Entry("JobBinDir(jobName)", p.JobBinDir("myJob"), "/some/dir/jobs/myJob/bin"),
		Entry("JobLogDir(jobName)", p.JobLogDir("myJob"), "/some/dir/data/sys/log/myJob"),
		Entry("JobRunDir(jobName)", p.JobRunDir("myJob"), "/some/dir/data/sys/run/myJob"),
		Entry("JobDir(jobName)", p.JobDir("myJob"), "/some/dir/data/myJob"),
		Entry("SettingsDir()", p.SettingsDir(), "/some/dir/bosh/settings"),
		Entry("TmpDir()", p.TmpDir(), "/some/dir/data/tmp"),
		Entry("CanRestartDir()", p.CanRestartDir(), "/some/dir/bosh/canrestart"),
		Entry("LogsDir()", p.LogsDir(), "/some/dir/sys/log"),
		Entry("AgentLogsDir()", p.AgentLogsDir(), "/some/dir/bosh/log"),
		Entry("InstanceDir()", p.InstanceDir(), "/some/dir/instance"),
		Entry("DisksDir()", p.DisksDir(), "/some/dir/instance/disks"),
		Entry("BlobsDir()", p.BlobsDir(), "/some/dir/data/blobs"),
		Entry("InstanceDNSDir()", p.InstanceDNSDir(), "/some/dir/instance/dns"),
	)

	It("cleans the base dir", func() {
		p := directories.NewProvider(filepath.FromSlash("/././/some/dir"))
		Expect(p.BaseDir()).To(Equal(filepath.FromSlash("/some/dir")))
	})
})
