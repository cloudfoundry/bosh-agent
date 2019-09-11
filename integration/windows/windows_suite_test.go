package windows_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"net/http"

	"github.com/cloudfoundry/bosh-agent/integration/windows/utils"
	"github.com/masterzen/winrm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"text/template"

	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
)

var (
	VagrantProvider             = os.Getenv("VAGRANT_PROVIDER")
	OsVersion                   = getOsVersion()
	AgentPublicIP, NATSPublicIP string
	dirname                     = filepath.Join(
		os.Getenv("GOPATH"),
		"src/github.com/cloudfoundry/bosh-agent/integration/windows/fixtures",
	)
	agent *WindowsEnvironment
)

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
		"OpenSSH-Win64.zip",
		"agent-configuration/agent.json",
		"agent-configuration/root-partition-agent.json",
		"agent-configuration/root-partition-agent-ephemeral-disabled.json",
		"agent-configuration/root-disk-settings.json",
		"agent-configuration/second-disk-settings.json",
		"agent-configuration/second-disk-digit-settings.json",
		"agent-configuration/third-disk-settings.json",
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
	if os.Getenv("GOPATH") == "" {
		Fail("Environment variable GOPATH not set", 1)
	}

	if err := utils.BuildAgent(); err != nil {
		Fail(fmt.Sprintln("Could not build the bosh-agent project.\nError is:", err))
	}

	err := utils.StartVagrant("nats", VagrantProvider, OsVersion)
	natsPrivateIP, err := utils.RetrievePrivateIP("nats")
	Expect(err).NotTo(HaveOccurred())
	Expect(natsPrivateIP).NotTo(BeEmpty(), "Couldn't retrieve NATS private IP")

	templateEphemeralDiskSettings(natsPrivateIP, `""`, "root-disk-settings.json")
	templateEphemeralDiskSettings(natsPrivateIP, `"/dev/sdb"`, "second-disk-settings.json")
	templateEphemeralDiskSettings(natsPrivateIP, `"1"`, "second-disk-digit-settings.json")
	templateEphemeralDiskSettings(natsPrivateIP, `{"path": "/dev/sdc"}`, "third-disk-settings.json")

	filename := filepath.Join(dirname, "fixtures.tgz")
	if err := tarFixtures(dirname, filename); err != nil {
		Fail(fmt.Sprintln("Creating fixtures TGZ::", err))
	}

	err = utils.StartVagrant("agent", VagrantProvider, OsVersion)

	AgentPublicIP, err = utils.RetrievePublicIP("agent")
	Expect(err).NotTo(HaveOccurred())

	NATSPublicIP, err = utils.RetrievePublicIP("nats")
	Expect(err).NotTo(HaveOccurred())

	if err != nil {
		Fail(fmt.Sprintln("Could not setup and run vagrant.\nError is:", err))
	}

	endpoint := winrm.NewEndpoint(AgentPublicIP, 5985, false, false, nil, nil, nil, 0)
	client, err := winrm.NewClientWithParameters(
		endpoint,
		"vagrant",
		"Password123!",
		winrm.NewParameters("PT5M", "en-US", 153600),
	)
	Expect(err).NotTo(HaveOccurred())

	agent = &WindowsEnvironment{
		Client: client,
		Linker: &disk.Linker{
			Runner: &utils.WinRMCommandRunner{
				Client: client,
			},
		},
	}

	// We do this so that both 2012R2 and 1709 run ephemeral disk tests against raw disks.
	// 2012R2 additional disks start formatted on AWS for some reason.
	agent.EnsureDiskCleared("1")
	agent.EnsureDiskCleared("2")

	goSourcePath := filepath.Join(dirname, "templates", "go", "go1.7.1.windows-amd64.zip")
	os.RemoveAll(goSourcePath)
	downloadFile(goSourcePath, "https://dl.google.com/go/go1.7.1.windows-amd64.zip")
	agent.RunPowershellCommand("add-content \\ProgramData\\ssh\\sshd_config \"AllowUsers bosh_testuser\"")
})

func templateEphemeralDiskSettings(natsPrivateIP, ephemeralDiskConfig, filename string) {
	agentSettings := BoshAgentSettings{
		NatsPrivateIP:       natsPrivateIP,
		EphemeralDiskConfig: ephemeralDiskConfig,
	}
	settingsTmpl, err := template.ParseFiles(
		filepath.Join(dirname, "templates", "agent-configuration", "settings.json.tmpl"),
	)
	Expect(err).NotTo(HaveOccurred())
	outputFile, err := os.Create(filepath.Join(dirname, "agent-configuration", filename))
	defer outputFile.Close()

	Expect(err).NotTo(HaveOccurred())
	err = settingsTmpl.Execute(outputFile, agentSettings)
	Expect(err).NotTo(HaveOccurred())
}

func downloadFile(localPath, sourceURL string) error {
	f, err := os.OpenFile(localPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	res, err := http.Get(sourceURL)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if _, err := io.Copy(f, res.Body); err != nil {
		return err
	}

	return nil
}
