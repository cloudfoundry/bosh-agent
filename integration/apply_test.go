package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"path/filepath"
	"time"

	"github.com/cloudfoundry/bosh-agent/agentclient"
	"github.com/cloudfoundry/bosh-agent/agentclient/applyspec"
	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("apply", func() {
	var (
		agentClient      agentclient.AgentClient
		registrySettings settings.Settings
		applySpec        applyspec.ApplySpec
	)

	BeforeEach(func() {
		err := testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.SetupConfigDrive()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("config-drive-agent.json")
		Expect(err).ToNot(HaveOccurred())

		registrySettings = settings.Settings{
			AgentID: "fake-agent-id",

			// note that this SETS the username and password for HTTP message bus access
			Mbus: "https://mbus-user:mbus-pass@127.0.0.1:6868",

			Blobstore: settings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					// Ignored because we rely on the BlobManagers in the
					// CascadingBlobstore to return blobs rather than the local
					// blobstore.
					"blobstore_path": "ignored",
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		applySpec = applyspec.ApplySpec{
			ConfigurationHash: "fake-desired-config-hash",
			NodeID:            "node-id01-123f-r2344",
			AvailabilityZone:  "ex-az",
			Deployment:        "deployment-name",
			Name:              "instance-name",

			Job: applyspec.Job{
				Name: "foobar-ig",
				Templates: []applyspec.Blob{
					{Name: "foobar", SHA1: "b70d2e6fefb1ff48f33a1cb08a609f19dd0f2c7d", BlobstoreID: "abc0", Version: "1234"},
				},
			},

			RenderedTemplatesArchive: applyspec.RenderedTemplatesArchiveSpec{
				BlobstoreID: "abc0",
				SHA1:        "b70d2e6fefb1ff48f33a1cb08a609f19dd0f2c7d",
			},

			Packages: map[string]applyspec.Blob{
				"bar": {
					Name:        "bar",
					SHA1:        "e2b85b98dcd20e9738f8db9e33dde65de2d623a4",
					BlobstoreID: "abc1",
					Version:     "1",
				},
				"foo": {
					Name:        "foo",
					SHA1:        "78848d5018c93761a48bdd35f221a15d28ff7a5e",
					BlobstoreID: "abc2",
					Version:     "1",
				},
			},
		}
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())

		agentClient, err = testEnvironment.StartAgentTunnel("mbus-user", "mbus-pass", 6868)
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.CreateSensitiveBlobFromAsset(filepath.Join("release", "jobs", "foobar.tgz"), "abc0")
		Expect(err).NotTo(HaveOccurred())
		err = testEnvironment.CreateBlobFromAsset(filepath.Join("release", "packages/bar.tgz"), "abc1")
		Expect(err).NotTo(HaveOccurred())
		err = testEnvironment.CreateBlobFromAsset(filepath.Join("release", "packages/foo.tgz"), "abc2")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should send agent apply and create appropriate /var/vcap/data directories for a job", func() {
		err := agentClient.Apply(applySpec)
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand("sudo stat /var/vcap/data/sys/run/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0770/drwxrwx---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/sys/log/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0770/drwxrwx---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0770/drwxrwx---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/packages")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages/bar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages/foo")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/jobs")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/jobs/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo bash -c 'stat /var/vcap/data/jobs/foobar/*'")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		err = testEnvironment.StopAgent()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			_, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/sys/run/foobar")
			return err
		}, 2*time.Minute, 1*time.Second).ShouldNot(HaveOccurred())

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/sys/run/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0770/drwxrwx---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/sys/log/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0770/drwxrwx---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

		output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/foobar")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("Access: \\(0770/drwxrwx---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))
	})
})
