package integration_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agentclient/applyspec"
	"github.com/cloudfoundry/bosh-agent/settings"
)

func setupDummyJob() {
	_, err := testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data/sensitive_blobs")
	Expect(err).NotTo(HaveOccurred())

	err = testEnvironment.CreateSensitiveBlobFromAsset(filepath.Join("release", "jobs/foobar.tgz"), "abc0")
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

	err = testEnvironment.AgentClient.Apply(applySpec)
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("run_script", func() {
	var (
		fileSettings settings.Settings
	)

	JustBeforeEach(func() {
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		err := testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		fileSettings = settings.Settings{
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

		err = testEnvironment.CreateSettingsFile(fileSettings)
		Expect(err).ToNot(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data/foobar")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	It("runs a custom script", func() {
		setupDummyJob()

		_, err := testEnvironment.RunCommand("sudo mkdir -p /var/vcap/jobs/foobar/bin")
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand(`sudo bash -c "echo '#!/bin/bash
		echo -n foobar > /var/vcap/data/foobar/output.txt' > /var/vcap/jobs/foobar/bin/custom-script"`)
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo chmod +x /var/vcap/jobs/foobar/bin/custom-script")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.AgentClient.RunScript("custom-script", map[string]interface{}{})
		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand("sudo cat /var/vcap/data/foobar/output.txt")

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("foobar"))
	})

	It("runs a custom script with variables", func() {

		setupDummyJob()

		_, err := testEnvironment.RunCommand("sudo mkdir -p /var/vcap/jobs/foobar/bin")
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand(`sudo bash -c "echo '#!/bin/bash
		echo -n \$FOO > /var/vcap/data/foobar/output.txt' > /var/vcap/jobs/foobar/bin/custom-script"`)
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo chmod +x /var/vcap/jobs/foobar/bin/custom-script")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.AgentClient.RunScript("custom-script", map[string]interface{}{
			"env": map[string]string{
				"FOO": "bar",
			},
		})

		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand("sudo cat /var/vcap/data/foobar/output.txt")

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("bar"))
	})
})
