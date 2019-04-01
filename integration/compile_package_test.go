package integration_test

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agentclient"
	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("compile_package", func() {
	var (
		agentClient      agentclient.AgentClient
		registrySettings settings.Settings
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

			Env: settings.Env{
				Bosh: settings.BoshEnv{
					TargetedBlobstores: settings.TargetedBlobstores{
						Packages: "custom-blobstore",
						Logs:     "custom-blobstore",
					},
					Blobstores: []settings.Blobstore{
						settings.Blobstore{
							Type: "local",
							Name: "ignored-blobstore",
							Options: map[string]interface{}{
								"blobstore_path": "/ignored/blobstore",
							},
						},
						settings.Blobstore{
							Type: "local",
							Name: "special-case-local-blobstore",
							Options: map[string]interface{}{
								// this path should get rewritten internally to /var/vcap/data/blobs
								"blobstore_path": "/var/vcap/micro_bosh/data/cache",
							},
						},
						settings.Blobstore{
							Type: "local",
							Name: "custom-blobstore",
							Options: map[string]interface{}{
								"blobstore_path": "/tmp/my-blobs",
							},
						},
					},
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())

		agentClient, err = testEnvironment.StartAgentTunnel("mbus-user", "mbus-pass", 6868)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())

		output, err := testEnvironment.RunCommand("sudo rm -rf /tmp/my-blobs")
		Expect(err).NotTo(HaveOccurred(), output)
	})

	Context("when micro_bosh is configured as the blobstore", func() {

		BeforeEach(func() {
			registrySettings.Env.Bosh.TargetedBlobstores.Packages = "special-case-local-blobstore"
		})

		It("compiles and stores it to the ephemeral disk", func() {
			blobId, err := uuid.NewV4()
			Expect(err).NotTo(HaveOccurred())

			// err = testEnvironment.CreateBlobFromAssetInActualBlobstore("dummy_package.tgz", "/tmp/my-blobs", blobId.String())
			err = testEnvironment.CreateBlobFromAsset("dummy_package.tgz", blobId.String())
			Expect(err).NotTo(HaveOccurred())

			result, err := agentClient.CompilePackage(agentclient.BlobRef{
				Name:        "fake",
				Version:     "1",
				BlobstoreID: blobId.String(),
				SHA1:        "236cbd31a483c3594061b00a84a80c1c182b3b20",
			}, []agentclient.BlobRef{})

			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat /var/vcap/data/blobs/%s", result.BlobstoreID))
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("regular file"))
		})
	})

	It("allows passing bare sha1 for legacy support", func() {
		blobId, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.CreateBlobFromAssetInActualBlobstore("dummy_package.tgz", "/tmp/my-blobs", blobId.String())
		Expect(err).NotTo(HaveOccurred())

		result, err := agentClient.CompilePackage(agentclient.BlobRef{
			Name:        "fake",
			Version:     "1",
			BlobstoreID: blobId.String(),
			SHA1:        "236cbd31a483c3594061b00a84a80c1c182b3b20",
		}, []agentclient.BlobRef{})

		Expect(err).NotTo(HaveOccurred())

		output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat /tmp/my-blobs/%s", result.BlobstoreID))
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp("regular file"))

		output, err = testEnvironment.RunCommand(`sudo /bin/bash -c "zgrep 'dummy contents of dummy package file' /tmp/my-blobs/* | wc -l"`)
		Expect(err).NotTo(HaveOccurred(), output)
		// we expect both the original, uncompiled copy and the compiled copy of the package to exist
		Expect(strings.Trim(output, "\n")).To(Equal("2"))
	})

	It("does not skip verification when digest argument is missing", func() {
		err := testEnvironment.CreateBlobFromAsset("dummy_package.tgz", "123")
		Expect(err).NotTo(HaveOccurred())

		_, err = agentClient.CompilePackage(agentclient.BlobRef{
			Name:        "fake",
			Version:     "1",
			BlobstoreID: "123",
			SHA1:        "",
		}, []agentclient.BlobRef{})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("No digest algorithm found. Supported algorithms: sha1, sha256, sha512"))
	})

})
