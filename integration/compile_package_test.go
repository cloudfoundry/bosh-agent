package integration_test

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	boshcomp "github.com/cloudfoundry/bosh-agent/v2/agent/compiler"
	"github.com/cloudfoundry/bosh-agent/v2/agentclient"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("compile_package", func() {
	var (
		fileSettings settings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		fileSettings = settings.Settings{
			Blobstore: settings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					// this path should get rewritten internally to /var/vcap/data/blobs
					"blobstore_path": "/var/vcap/micro_bosh/data/cache",
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
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when configured with a signed URL", func() {
		var (
			dummyPackageSignedURL      string
			compiledDummyPackagePutURL string
			compiledPackagePath        string
		)

		multiDigest := createSHA1MultiDigest("236cbd31a483c3594061b00a84a80c1c182b3b20")

		BeforeEach(func() {
			err := testEnvironment.StartBlobstore()
			Expect(err).NotTo(HaveOccurred())

			err = testEnvironment.CopyFileToPath(filepath.Join(testEnvironment.AssetsDir(), "dummy_package.tgz"), filepath.Join(testEnvironment.BlobstoreDir(), "dummy_package.tgz"))
			Expect(err).NotTo(HaveOccurred())
			dummyPackageSignedURL = "http://127.0.0.1:9091/get_package/dummy_package.tgz"

			id, err := uuid.NewV4()
			Expect(err).NotTo(HaveOccurred())

			compiledPackagePath = fmt.Sprintf("compiled-dummy-packages-%s.tgz", id.String())
			compiledDummyPackagePutURL = fmt.Sprintf("http://127.0.0.1:9091/upload_package/%s", compiledPackagePath)
		})

		It("compiles and stores it to the blobstore", func() {
			_, err := testEnvironment.AgentClient.CompilePackageWithSignedURL(action.CompilePackageWithSignedURLRequest{
				PackageGetSignedURL: dummyPackageSignedURL,
				UploadSignedURL:     compiledDummyPackagePutURL,

				Digest:  multiDigest,
				Name:    "fake",
				Version: "1",
				Deps:    boshcomp.Dependencies{},
			})
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s", filepath.Join(testEnvironment.BlobstoreDir(), compiledPackagePath)))
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("regular file"))
		})

		It("allows passing bare sha1 for legacy support", func() {
			_, err := testEnvironment.AgentClient.CompilePackageWithSignedURL(action.CompilePackageWithSignedURLRequest{
				Name:                "fake",
				Version:             "1",
				PackageGetSignedURL: dummyPackageSignedURL,
				UploadSignedURL:     compiledDummyPackagePutURL,
				Digest:              multiDigest,
				Deps:                boshcomp.Dependencies{},
			})
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s", filepath.Join(testEnvironment.BlobstoreDir(), compiledPackagePath)))
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("regular file"))
		})

		It("does not skip verification when digest argument is missing", func() {
			_, err := testEnvironment.AgentClient.CompilePackageWithSignedURL(action.CompilePackageWithSignedURLRequest{
				Name:                "fake",
				Version:             "1",
				PackageGetSignedURL: dummyPackageSignedURL,
				UploadSignedURL:     compiledDummyPackagePutURL,
				Digest:              createSHA1MultiDigest(""),
			})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No digest algorithm found. Supported algorithms: sha1, sha256, sha512"))
		})

		It("compiles dependencies and stores them to the blobstore", func() {
			_, err := testEnvironment.AgentClient.CompilePackageWithSignedURL(action.CompilePackageWithSignedURLRequest{
				PackageGetSignedURL: dummyPackageSignedURL,
				UploadSignedURL:     compiledDummyPackagePutURL,
				Digest:              multiDigest,
				Name:                "fake",
				Version:             "1",
				Deps: boshcomp.Dependencies{"fake-dep-1": boshcomp.Package{
					Name:                "fake-dep-1",
					PackageGetSignedURL: dummyPackageSignedURL,
					UploadSignedURL:     compiledDummyPackagePutURL,
					Sha1:                multiDigest,
					Version:             "1",
				}}})
			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat %s", filepath.Join(testEnvironment.BlobstoreDir(), compiledPackagePath)))
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("regular file"))
		})
	})

	Context("when configured with a blobstore", func() {
		JustBeforeEach(func() {
			err := testEnvironment.CreateBlobFromAsset("dummy_package.tgz", "123")
			Expect(err).NotTo(HaveOccurred())
		})

		It("compiles and stores it to the blobstore", func() {
			result, err := testEnvironment.AgentClient.CompilePackage(agentclient.BlobRef{
				Name:        "fake",
				Version:     "1",
				BlobstoreID: "123",
				SHA1:        "236cbd31a483c3594061b00a84a80c1c182b3b20",
			}, []agentclient.BlobRef{})

			Expect(err).NotTo(HaveOccurred())

			output, err := testEnvironment.RunCommand(fmt.Sprintf("sudo stat /var/vcap/data/blobs/%s", result.BlobstoreID))
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchRegexp("regular file"))
		})

		It("allows passing bare sha1 for legacy support", func() {
			_, err := testEnvironment.AgentClient.CompilePackage(agentclient.BlobRef{
				Name:        "fake",
				Version:     "1",
				BlobstoreID: "123",
				SHA1:        "236cbd31a483c3594061b00a84a80c1c182b3b20",
			}, []agentclient.BlobRef{})

			Expect(err).NotTo(HaveOccurred())

			out, err := testEnvironment.RunCommand(`sudo /bin/bash -c "zgrep 'dummy contents of dummy package file' /var/vcap/data/blobs/* 2>&1 | wc -l"`)
			Expect(err).NotTo(HaveOccurred(), out)
			// we expect both the original, uncompiled copy and the compiled copy of the package to exist
			Expect(strings.Trim(out, "\n")).To(Equal("2"))

		})

		It("does not skip verification when digest argument is missing", func() {
			_, err := testEnvironment.AgentClient.CompilePackage(agentclient.BlobRef{
				Name:        "fake",
				Version:     "1",
				BlobstoreID: "123",
				SHA1:        "",
			}, []agentclient.BlobRef{})

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No digest algorithm found. Supported algorithms: sha1, sha256, sha512"))
		})
	})
})

func createSHA1MultiDigest(digest string) boshcrypto.MultipleDigest {
	return boshcrypto.MustNewMultipleDigest(
		boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, digest))
}
