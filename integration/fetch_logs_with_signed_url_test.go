package integration_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/integration/integrationagentclient"
	"github.com/cloudfoundry/bosh-agent/settings"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/http_blob_provider"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

func generateSignedURL(bucket, key string) string {
	sess, err := session.NewSession(&aws.Config{})
	Expect(err).NotTo(HaveOccurred())

	svc := s3.New(sess)

	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	urlStr, err := req.Presign(15 * time.Minute)
	Expect(err).NotTo(HaveOccurred())

	return urlStr
}

func downloadS3ObjectContents(bucket, key string) ([]byte, string) {
	sess, err := session.NewSession(&aws.Config{})
	Expect(err).NotTo(HaveOccurred())

	svc := s3.New(sess)
	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	Expect(err).NotTo(HaveOccurred())

	var bodyCopy bytes.Buffer
	_, err = io.Copy(&bodyCopy, out.Body)
	Expect(err).ToNot(HaveOccurred())

	// tee := io.TeeReader(out.Body, &bodyCopy)
	// bodyCopy := bytes.NewReader(out.Body)
	// _, err = io.Copy(&bodyCopy, tee)
	// Expect(err).ToNot(HaveOccurred())

	multidigest, err := boshcrypto.NewMultipleDigest(bytes.NewReader(bodyCopy.Bytes()),
		httpblobprovider.DefaultCryptoAlgorithms)
	Expect(err).NotTo(HaveOccurred())

	r, err := gzip.NewReader(&bodyCopy)
	if err != nil {
		panic(err)
	}
	defer r.Close()
	contents, err := ioutil.ReadAll(r)
	Expect(err).NotTo(HaveOccurred())

	return contents, multidigest.String()
}

func removeS3Object(bucket, key string) {
	sess, err := session.NewSession(&aws.Config{})
	Expect(err).NotTo(HaveOccurred())

	svc := s3.New(sess)
	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	Expect(err).NotTo(HaveOccurred())
}

var _ = Describe("fetch_logs_with_signed_url", func() {
	var (
		agentClient      *integrationagentclient.IntegrationAgentClient
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
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	var (
		bucket string
		key    string
	)

	BeforeEach(func() {
		Expect(os.Getenv("AWS_ACCESS_KEY")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_REGION")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_BUCKET")).NotTo(BeEmpty())

		bucket = os.Getenv("AWS_BUCKET")
		key = "s3-signed-file.txt"
	})

	AfterEach(func() {
		removeS3Object(bucket, key)
	})

	It("puts the logs in the appropriate blobstore location", func() {
		r, stderr, _, err := testEnvironment.RunCommand3("echo 'foobarbaz' | sudo tee /var/vcap/sys/log/fetch-logs")
		Expect(err).NotTo(HaveOccurred(), r, stderr)

		signedURL := generateSignedURL(bucket, key)

		response, err := agentClient.FetchLogsWithSignedURLAction(signedURL, "job", nil)
		Expect(err).NotTo(HaveOccurred())

		contents, sha1 := downloadS3ObjectContents(bucket, key)
		Expect(response.SHA1Digest).To(Equal(sha1))

		Expect(contents).To(ContainSubstring("foobarbaz"))
		Expect(contents).To(ContainSubstring("fetch-logs"))
	})
})
