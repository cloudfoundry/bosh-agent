package integration_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/cloudfoundry/bosh-agent/integration"
)

var (
	testEnvironment *TestEnvironment
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)

	BeforeSuite(func() {
		logger := boshlog.NewLogger(boshlog.LevelDebug)
		cmdRunner := boshsys.NewExecCmdRunner(logger)
		var err error
		testEnvironment, err = NewTestEnvironment(cmdRunner)
		Expect(err).ToNot(HaveOccurred())

		// Required for reverse-compatibility with older bosh-lite
		// (remove once a new warden stemcell is built).
		err = testEnvironment.ConfigureAgentForGenericInfrastructure()
		Expect(err).ToNot(HaveOccurred())

		// Required for signed url tests
		Expect(os.Getenv("AWS_ACCESS_KEY")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_REGION")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_BUCKET")).NotTo(BeEmpty())
	})

	AfterEach(func() {
		err := testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.DetachLoopDevices()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.ResetDeviceMap()
		Expect(err).ToNot(HaveOccurred())
	})

	RunSpecs(t, "Integration Suite")
}

func generateSignedURLForPut(bucket, key string) string {
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

func generateSignedURLForGet(bucket, key string) string {
	sess, err := session.NewSession(&aws.Config{})
	Expect(err).NotTo(HaveOccurred())

	svc := s3.New(sess)

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	urlStr, err := req.Presign(15 * time.Minute)
	Expect(err).NotTo(HaveOccurred())

	return urlStr
}

func uploadS3Object(bucket, key string, r io.Reader) {
	sess, err := session.NewSession(&aws.Config{})
	Expect(err).NotTo(HaveOccurred())

	uploader := s3manager.NewUploader(sess)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	Expect(err).NotTo(HaveOccurred())
}
