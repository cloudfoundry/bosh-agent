package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/integration/integrationagentclient"
	"github.com/cloudfoundry/bosh-agent/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("v1_apply", func() {
	var (
		agentClient      *integrationagentclient.IntegrationAgentClient
		registrySettings settings.Settings
		applySpec        applyspec.V1ApplySpec
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
	})

	Context("when configured with a signed urls", func() {
		var (
			barPackageSignedURL string
			fooPackageSignedURL string
			foobarJobSignedURL  string
			s3Bucket            string
		)

		AfterEach(func() {
			removeS3Object(s3Bucket, "foobar.tgz")
			removeS3Object(s3Bucket, "foo.tgz")
			removeS3Object(s3Bucket, "bar.tgz")
		})

		BeforeEach(func() {
			s3Bucket = os.Getenv("AWS_BUCKET")
			foobarReader, err := os.Open(filepath.Join("assets", "release", "jobs", "foobar.tgz"))
			defer foobarReader.Close()
			Expect(err).NotTo(HaveOccurred())
			uploadS3Object(s3Bucket, "foobar.tgz", foobarReader)

			barReader, err := os.Open(filepath.Join("assets", "release", "packages", "bar.tgz"))
			defer barReader.Close()
			Expect(err).NotTo(HaveOccurred())
			uploadS3Object(s3Bucket, "bar.tgz", barReader)

			fooReader, err := os.Open(filepath.Join("assets", "release", "packages", "foo.tgz"))
			defer fooReader.Close()
			Expect(err).NotTo(HaveOccurred())
			uploadS3Object(s3Bucket, "foo.tgz", fooReader)

			foobarJobSignedURL = generateSignedURLForGet(s3Bucket, "foobar.tgz")
			fooPackageSignedURL = generateSignedURLForGet(s3Bucket, "foo.tgz")
			barPackageSignedURL = generateSignedURLForGet(s3Bucket, "bar.tgz")

			stringPointerFunc := func(a string) *string {
				return &a
			}

			getDigest := func(shasum string) *boshcrypto.MultipleDigest {
				sha1Digest := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, shasum)
				digest := boshcrypto.MustNewMultipleDigest(sha1Digest)
				return &digest
			}

			applySpec = applyspec.V1ApplySpec{
				ConfigurationHash: "fake-desired-config-hash",
				NodeID:            "node-id01-123f-r2344",
				AvailabilityZone:  "ex-az",
				Deployment:        "deployment-name",
				Name:              "instance-name",

				JobSpec: applyspec.JobSpec{
					Name: stringPointerFunc("foobar-ig"),
					JobTemplateSpecs: []applyspec.JobTemplateSpec{
						applyspec.JobTemplateSpec{
							Name:    "foobar",
							Version: "1234",
						},
					},
				},

				RenderedTemplatesArchiveSpec: &applyspec.RenderedTemplatesArchiveSpec{
					SignedURL: foobarJobSignedURL,
					Sha1:      getDigest("b70d2e6fefb1ff48f33a1cb08a609f19dd0f2c7d"),
				},

				PackageSpecs: map[string]applyspec.PackageSpec{
					"bar": {
						Name:      "bar",
						Sha1:      *getDigest("e2b85b98dcd20e9738f8db9e33dde65de2d623a4"),
						SignedURL: barPackageSignedURL,
						Version:   "1",
					},
					"foo": {
						Name:      "foo",
						Sha1:      *getDigest("78848d5018c93761a48bdd35f221a15d28ff7a5e"),
						SignedURL: fooPackageSignedURL,
						Version:   "1",
					},
				},
			}
		})

		It("should send agent apply and create appropriate /var/vcap/data directories for a job", func() {
			err := agentClient.Prepare(applySpec)
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand("stat /var/vcap/packages")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/packages")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/packages/bar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/packages/foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/jobs")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/jobs/foobar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/jobs/foobar/*")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))
		})
	})

	Context("when configured with a blobstore", func() {
		BeforeEach(func() {
			stringPointerFunc := func(a string) *string {
				return &a
			}
			getDigest := func(shasum string) *boshcrypto.MultipleDigest {
				sha1Digest := boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, shasum)
				digest := boshcrypto.MustNewMultipleDigest(sha1Digest)
				return &digest
			}

			applySpec = applyspec.V1ApplySpec{
				ConfigurationHash: "fake-desired-config-hash",
				NodeID:            "node-id01-123f-r2344",
				AvailabilityZone:  "ex-az",
				Deployment:        "deployment-name",
				Name:              "instance-name",

				JobSpec: applyspec.JobSpec{
					Name: stringPointerFunc("foobar-ig"),
					JobTemplateSpecs: []applyspec.JobTemplateSpec{
						applyspec.JobTemplateSpec{
							Name:    "foobar",
							Version: "1234",
						},
					},
				},

				RenderedTemplatesArchiveSpec: &applyspec.RenderedTemplatesArchiveSpec{
					BlobstoreID: "abc0",
					Sha1:        getDigest("b70d2e6fefb1ff48f33a1cb08a609f19dd0f2c7d"),
				},

				PackageSpecs: map[string]applyspec.PackageSpec{
					"bar": {
						Name:        "bar",
						Sha1:        *getDigest("e2b85b98dcd20e9738f8db9e33dde65de2d623a4"),
						BlobstoreID: "abc1",
						Version:     "1",
					},
					"foo": {
						Name:        "foo",
						Sha1:        *getDigest("78848d5018c93761a48bdd35f221a15d28ff7a5e"),
						BlobstoreID: "abc2",
						Version:     "1",
					},
				},
			}
		})

		JustBeforeEach(func() {
			_, err := testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
			Expect(err).NotTo(HaveOccurred())

			err = testEnvironment.CreateSensitiveBlobFromAsset(filepath.Join("release", "jobs/foobar.tgz"), "abc0")
			Expect(err).NotTo(HaveOccurred())
			err = testEnvironment.CreateBlobFromAsset(filepath.Join("release", "packages/bar.tgz"), "abc1")
			Expect(err).NotTo(HaveOccurred())
			err = testEnvironment.CreateBlobFromAsset(filepath.Join("release", "packages/foo.tgz"), "abc2")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should send agent apply and create appropriate /var/vcap/data directories for a job", func() {
			err := agentClient.Prepare(applySpec)
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand("stat /var/vcap/packages")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/packages")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/packages/bar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/packages/foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/jobs")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/jobs/foobar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("stat /var/vcap/data/jobs/foobar/*")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))
		})

		Context("when settings tmpfs is enabled", func() {
			BeforeEach(func() {
				registrySettings.Env.Bosh.Agent.Settings.TmpFS = true
			})

			It("mounts a tmpfs for /var/vcap/settings", func() {
				err := agentClient.Prepare(applySpec)
				Expect(err).NotTo(HaveOccurred())

				err = agentClient.AddPersistentDisk("disk-cid", "/dev/sdf")
				Expect(err).NotTo(HaveOccurred())

				output, err := testEnvironment.RunCommand("sudo cat /proc/mounts")
				Expect(err).NotTo(HaveOccurred())

				Expect(output).To(ContainSubstring("tmpfs /var/vcap/bosh/settings"))

				output, err = testEnvironment.RunCommand("sudo ls /var/vcap/bosh/settings")
				Expect(err).NotTo(HaveOccurred())
				Expect(output).To(ContainSubstring("settings.json"))
				Expect(output).To(ContainSubstring("persistent_disk_hints.json"))

				_, err = testEnvironment.RunCommand("sudo umount /var/vcap/bosh/settings")
				Expect(err).NotTo(HaveOccurred())

				output, err = testEnvironment.RunCommand("sudo ls /var/vcap/bosh/settings")
				Expect(err).NotTo(HaveOccurred())
				Expect(output).NotTo(ContainSubstring("settings.json"))
				Expect(output).NotTo(ContainSubstring("persistent_disk_hints.json"))
			})
		})

		Context("when job dir tmpfs is enabled", func() {
			BeforeEach(func() {
				registrySettings.Env.Bosh.JobDir.TmpFS = true
			})

			It("mounts a tmpfs for /var/vcap/data/jobs", func() {
				err := agentClient.Prepare(applySpec)
				Expect(err).NotTo(HaveOccurred())

				output, err := testEnvironment.RunCommand("sudo cat /proc/mounts")
				Expect(err).NotTo(HaveOccurred())

				Expect(output).To(ContainSubstring("tmpfs /var/vcap/data/jobs"))
				Expect(output).To(ContainSubstring("tmpfs /var/vcap/data/sensitive_blobs"))
			})
		})
	})
})
