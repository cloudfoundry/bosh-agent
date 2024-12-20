package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("prepare", func() {
	var (
		fileSettings settings.Settings
		applySpec    applyspec.V1ApplySpec
	)

	BeforeEach(func() {
		err := testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		fileSettings = settings.Settings{
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
		err = testEnvironment.CreateSettingsFile(fileSettings)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {

		// StartAgentTunnel also acts as a wait condition for the agent to have fully started. Copying over the blobs before it fully starts, will result in issues because the agent cleans up dirs on start.

		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.CreateBlobFromAsset(filepath.Join("release", "packages/bar.tgz"), "abc1")
		Expect(err).NotTo(HaveOccurred())
		err = testEnvironment.CreateBlobFromAsset(filepath.Join("release", "packages/foo.tgz"), "abc2")
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.CreateSensitiveBlobFromAsset(filepath.Join("release", "jobs/foobar.tgz"), "abc0")
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when configured with signed urls", func() {
		var (
			barPackageSignedURL string
			fooPackageSignedURL string
		)

		BeforeEach(func() {
			fooPackageSignedURL = "http://127.0.0.1:9091/get_package/release/packages/foo.tgz?encrypted"
			barPackageSignedURL = "http://127.0.0.1:9091/get_package/release/packages/bar.tgz?encrypted"

			encryptionHeaders := map[string]string{
				"encryption-key": "value",
			}

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
						{
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
						Name:             "bar",
						Sha1:             *getDigest("e2b85b98dcd20e9738f8db9e33dde65de2d623a4"),
						SignedURL:        barPackageSignedURL,
						BlobstoreHeaders: encryptionHeaders,
						Version:          "1",
					},
					"foo": {
						Name:             "foo",
						Sha1:             *getDigest("78848d5018c93761a48bdd35f221a15d28ff7a5e"),
						SignedURL:        fooPackageSignedURL,
						BlobstoreHeaders: encryptionHeaders,
						Version:          "1",
					},
				},
			}

			err := testEnvironment.StartBlobstore()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should send agent apply and create appropriate /var/vcap/data directories for a job", func() {
			err := testEnvironment.AgentClient.Prepare(applySpec)
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand("sudo stat /var/vcap/data/packages")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages/bar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages/foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/jobs/foobar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo bash -c 'stat /var/vcap/data/jobs/foobar/*'")
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
						{
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

		It("should send agent apply and create appropriate /var/vcap/data directories for a job", func() {
			err := testEnvironment.AgentClient.Prepare(applySpec)
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand("sudo stat /var/vcap/data/packages")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages/bar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/packages/foo")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0755/drwxr-xr-x\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo stat /var/vcap/data/jobs/foobar")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))

			output, err = testEnvironment.RunCommand("sudo bash -c 'stat /var/vcap/data/jobs/foobar/*'")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("Access: \\(0750/drwxr-x---\\)  Uid: \\(    0/    root\\)   Gid: \\( 100[0-9]/    vcap\\)"))
		})

		Context("when settings tmpfs is enabled", func() {
			BeforeEach(func() {
				fileSettings.Env.Bosh.Agent.Settings.TmpFS = true
				err := testEnvironment.CreateSettingsFile(fileSettings)
				Expect(err).ToNot(HaveOccurred())
			})

			It("mounts a tmpfs for /var/vcap/settings", func() {
				err := testEnvironment.AgentClient.Prepare(applySpec)
				Expect(err).NotTo(HaveOccurred())

				err = testEnvironment.AgentClient.AddPersistentDisk("disk-cid", "/dev/sdf")
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
				fileSettings.Env.Bosh.JobDir.TmpFS = true
				err := testEnvironment.CreateSettingsFile(fileSettings)
				Expect(err).ToNot(HaveOccurred())
			})

			It("mounts a tmpfs for /var/vcap/data/jobs", func() {
				err := testEnvironment.AgentClient.Prepare(applySpec)
				Expect(err).NotTo(HaveOccurred())

				output, err := testEnvironment.RunCommand("sudo cat /proc/mounts")
				Expect(err).NotTo(HaveOccurred())

				Expect(output).To(ContainSubstring("tmpfs /var/vcap/data/jobs"))
				Expect(output).To(ContainSubstring("tmpfs /var/vcap/data/sensitive_blobs"))
			})
		})
	})
})
