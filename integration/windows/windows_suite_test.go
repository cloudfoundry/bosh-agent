package windows_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var VagrantProvider = os.Getenv("VAGRANT_PROVIDER")

func TestWindows(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Windows Suite")
}

func tarFixtures(filename string) error {
	fixtures := []string{
		"fixtures/service_wrapper.xml",
		"fixtures/service_wrapper.exe",
		"fixtures/job-service-wrapper.exe",
		"fixtures/bosh-blobstore-dav.exe",
		"fixtures/bosh-agent.exe",
		"fixtures/pipe.exe",
	}

	archive, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer archive.Close()

	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, name := range fixtures {
		fi, err := os.Stat(name)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}

		if err := tarWriter.WriteHeader(hdr); err != nil {
			return err
		}

		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tarWriter, f); err != nil {
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return nil
}

var _ = BeforeSuite(func() {
	if _, ok := os.LookupEnv("NATS_PRIVATE_IP"); !ok {
		Fail("Environment variable NATS_PRIVATE_IP not set (default is 172.31.180.3 if running locally)", 1)
	}
	if err := tarFixtures("fixtures/fixtures.tgz"); err != nil {
		Fail(fmt.Sprintln("Creating fixtures TGZ::", err))
	}
	_, err := utils.StartVagrant(VagrantProvider)
	if err != nil {
		Fail(fmt.Sprintln("Could not build the bosh-agent project.\nError is:", err))
	}
})
