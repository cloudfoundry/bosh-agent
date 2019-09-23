package integration_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agentclient"
	"github.com/cloudfoundry/bosh-agent/agentclient/applyspec"
	"github.com/cloudfoundry/bosh-agent/settings"
)

func setupDummyJob(agentClient agentclient.AgentClient) {
	err := testEnvironment.CreateSensitiveBlobFromAsset(filepath.Join("release", "jobs/foobar.tgz"), "abc0")
	Expect(err).NotTo(HaveOccurred())

	applySpec := applyspec.ApplySpec{
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

		Packages: map[string]applyspec.Blob{},
	}

	err = agentClient.Apply(applySpec)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("run_script", func() {
	var (
		agentClient      agentclient.AgentClient
		registrySettings settings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.StopAgent()
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
					"blobstore_path": "/var/vcap/data/blobs",
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())

		agentClient, err = testEnvironment.StartAgentTunnel("mbus-user", "mbus-pass", 6868)
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())

		setupDummyJob(agentClient)
	})

	BeforeEach(func() {
		r, stderr, _, err := testEnvironment.RunCommand3("rm -f /var/vcap/data/foobar/output.txt")
		Expect(err).NotTo(HaveOccurred(), r, stderr)

		r, stderr, _, err = testEnvironment.RunCommand3("sudo mkdir -p /var/vcap/jobs/foobar/bin")
		Expect(err).NotTo(HaveOccurred(), r, stderr)

		r, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data/foobar")
		Expect(err).NotTo(HaveOccurred(), r)
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	It("runs a custom script", func() {
		r, stderr, _, err := testEnvironment.RunCommand3(`sudo bash -c "echo '#!/bin/bash
		echo -n foobar > /var/vcap/data/foobar/output.txt' > /var/vcap/jobs/foobar/bin/custom-script"`)
		Expect(err).NotTo(HaveOccurred(), r, stderr)

		r, err = testEnvironment.RunCommand("sudo chmod +x /var/vcap/jobs/foobar/bin/custom-script")
		Expect(err).NotTo(HaveOccurred(), r)

		err = agentClient.RunScript("custom-script", map[string]interface{}{})
		Expect(err).NotTo(HaveOccurred())

		r, stderr, _, err = testEnvironment.RunCommand3("cat /var/vcap/data/foobar/output.txt")

		Expect(err).NotTo(HaveOccurred(), r, stderr)
		Expect(r).To(Equal("foobar"))
	})

	It("runs a custom script with variables", func() {
		r, stderr, _, err := testEnvironment.RunCommand3(`sudo bash -c "echo '#!/bin/bash
		echo -n \$FOO > /var/vcap/data/foobar/output.txt' > /var/vcap/jobs/foobar/bin/custom-script"`)
		Expect(err).NotTo(HaveOccurred(), r, stderr)

		r, err = testEnvironment.RunCommand("sudo chmod +x /var/vcap/jobs/foobar/bin/custom-script")
		Expect(err).NotTo(HaveOccurred(), r)

		options := make(map[string]interface{})
		options["env"] = map[string]string{
			"FOO": "bar",
		}
		err = agentClient.RunScript("custom-script", options)
		Expect(err).NotTo(HaveOccurred())

		r, stderr, _, err = testEnvironment.RunCommand3("cat /var/vcap/data/foobar/output.txt")

		Expect(err).NotTo(HaveOccurred(), r, stderr)
		Expect(r).To(Equal("bar"))
	})
})
