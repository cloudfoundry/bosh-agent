package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// This test assumes the following environment variables have been set
// AWS_ACCESS_KEY
// AWS_SECRET_ACCESS_KEY
// AWS_REGION
// AWS_BUCKET

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

func downloadS3ObjectContents(bucket, key string) []byte {
	sess, err := session.NewSession(&aws.Config{})
	Expect(err).NotTo(HaveOccurred())

	svc := s3.New(sess)
	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	Expect(err).NotTo(HaveOccurred())

	contents, err := ioutil.ReadAll(out.Body)
	Expect(err).NotTo(HaveOccurred())

	return contents
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

var _ = Describe("S3 HTTP Blob Provider", func() {
	var (
		bucket       string
		key          string
		tmpfile      *os.File
		blobContents []byte
	)

	BeforeEach(func() {
		Expect(os.Getenv("AWS_ACCESS_KEY")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_SECRET_ACCESS_KEY")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_REGION")).NotTo(BeEmpty())
		Expect(os.Getenv("AWS_BUCKET")).NotTo(BeEmpty())

		bucket = os.Getenv("AWS_BUCKET")
		key = "s3-signed-file.txt"

		var err error
		blobContents = []byte(fmt.Sprintf("changing file contents %d", time.Now().Unix()))
		tmpfile, err = ioutil.TempFile("", "example")
		Expect(err).NotTo(HaveOccurred())

		_, err = tmpfile.Write(blobContents)
		Expect(err).NotTo(HaveOccurred())

		err = tmpfile.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(tmpfile.Name())

		removeS3Object(bucket, key)
	})

	It("successfully uploads a file via a signed URL", func() {
		signedURL := generateSignedURL(bucket, key)

		logger := boshlog.NewAsyncWriterLogger(boshlog.LevelNone, os.Stderr)
		realFileSystem := boshsys.NewOsFileSystem(logger)

		httpClient := httpclient.CreateDefaultClient(nil)
		blobProvider := httpblobprovider.NewHTTPBlobImplWithDigestAlgorithms(realFileSystem, httpClient, []boshcrypto.Algorithm{boshcrypto.DigestAlgorithmSHA512})

		_, err := blobProvider.Upload(signedURL, tmpfile.Name(), nil)
		Expect(err).NotTo(HaveOccurred())

		contents := downloadS3ObjectContents(bucket, key)
		Expect(contents).To(Equal(blobContents))
	})

	It("successfully downloads a file via signed URL", func() {
		Skip("Not implemented yet")
	})
})
