package windows_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"text/template"
)

var VagrantProvider = os.Getenv("VAGRANT_PROVIDER")
var OsVersion = getOsVersion()

type BoshAgentSettings struct {
	NatsPrivateIP       string
	EphemeralDiskConfig string
}

func getOsVersion() string {
	osVersion := os.Getenv("WINDOWS_OS_VERSION")
	if osVersion == "" {
		osVersion = "2012R2"
	}

	return osVersion
}

func TestWindows(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Windows Suite")
}

func tarFixtures(fixturesDir, filename string) error {
	fixtures := []string{
		"service_wrapper.xml",
		"service_wrapper.exe",
		"job-service-wrapper.exe",
		"bosh-blobstore-dav.exe",
		"bosh-agent.exe",
		"pipe.exe",
		"agent-configuration/agent.json",
		"agent-configuration/root-partition-agent.json",
		"agent-configuration/root-partition-agent-ephemeral-disabled.json",
		"agent-configuration/settings.json",
		"psFixture/psFixture.psd1",
		"psFixture/psFixture.psm1",
	}

	archive, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer archive.Close()

	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, name := range fixtures {
		path := filepath.Join(fixturesDir, name)
		fi, err := os.Stat(path)
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = name

		if err := tarWriter.WriteHeader(hdr); err != nil {
			return err
		}

		f, err := os.Open(path)
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

	return gzipWriter.Close()
}

var _ = BeforeSuite(func() {
	natsPrivateIP, ok := os.LookupEnv("NATS_PRIVATE_IP")
	if !ok {
		Fail("Environment variable NATS_PRIVATE_IP not set (default is 172.31.180.3 if running locally)", 1)
	}
	if os.Getenv("GOPATH") == "" {
		Fail("Environment variable GOPATH not set", 1)
	}

	if err := utils.BuildAgent(); err != nil {
		Fail(fmt.Sprintln("Could not build the bosh-agent project.\nError is:", err))
	}

	dirname := filepath.Join(os.Getenv("GOPATH"),
		"src/github.com/cloudfoundry/bosh-agent/integration/windows/fixtures")

	agentSettings := BoshAgentSettings{
		NatsPrivateIP:       natsPrivateIP,
		EphemeralDiskConfig: `""`,
	}
	settingsTmpl, err := template.ParseFiles(
		filepath.Join(dirname, "templates", "agent-configuration", "settings.json.tmpl"),
	)
	Expect(err).NotTo(HaveOccurred())

	outputFile, err := os.Create(filepath.Join(dirname, "agent-configuration", "settings.json"))
	Expect(err).NotTo(HaveOccurred())
	defer outputFile.Close()

	err = settingsTmpl.Execute(outputFile, agentSettings)
	Expect(err).NotTo(HaveOccurred())

	filename := filepath.Join(dirname, "fixtures.tgz")
	if err := tarFixtures(dirname, filename); err != nil {
		Fail(fmt.Sprintln("Creating fixtures TGZ::", err))
	}

	_, err = utils.StartVagrant(VagrantProvider, OsVersion)

	if err != nil {
		Fail(fmt.Sprintln("Could not setup and run vagrant.\nError is:", err))
	}
})
