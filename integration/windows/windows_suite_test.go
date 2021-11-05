package windows_test

import (
	"fmt"
	"github.com/onsi/gomega/gexec"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	agent *WindowsEnvironment
)

type BoshAgentSettings struct {
	NatsPrivateIP       string
	EphemeralDiskConfig string
	AgentIP             string
	AgentNetmask        string
	AgentGateway        string
	NatsCA              string
	NatsCertificate     string
	NatsPrivateKey      string
}

func TestWindows(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Windows Suite")
}

var _ = BeforeSuite(func() {
	natsIP := utils.FakeDirectorIP()

	templateEphemeralDiskSettings(natsIP, `""`, "root-disk-settings.json")
	templateEphemeralDiskSettings(natsIP, `"/dev/sdb"`, "second-disk-settings.json")
	templateEphemeralDiskSettings(natsIP, `"1"`, "second-disk-digit-settings.json")
	templateEphemeralDiskSettings(natsIP, `{"path": "/dev/sdc"}`, "third-disk-settings.json")

	sshClient, err := utils.GetSSHTunnelClient()

	endpoint := winrm.NewEndpoint(utils.AgentIP(), 5985, false, false, nil, nil, nil, 0)

	params := winrm.NewParameters("PT5M", "en-US", 153600)
	params.Dial = sshClient.Dial

	client, err := winrm.NewClientWithParameters(
		endpoint,
		"vcap",
		"Agent-test-password1",
		params,
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

	agent.CleanUpExtraDisks()

	goSourcePath := filepath.Join(utils.AgentDir(), "integration", "windows", "fixtures", "templates", "go", "go1.7.1.windows-amd64.zip")
	os.RemoveAll(goSourcePath)
	downloadFile(goSourcePath, "https://dl.google.com/go/go1.7.1.windows-amd64.zip")
	//agent.RunPowershellCommand("add-content \\ProgramData\\ssh\\sshd_config \"AllowUsers bosh_testuser\"")
})

func templateEphemeralDiskSettings(natsPrivateIP, ephemeralDiskConfig, filename string) {
	agentSettings := BoshAgentSettings{
		NatsPrivateIP:       natsPrivateIP,
		EphemeralDiskConfig: ephemeralDiskConfig,
		AgentIP:             utils.AgentIP(),
		AgentNetmask:        utils.AgentNetmask(),
		AgentGateway:        utils.AgentGateway(),
		NatsCA:              strings.Replace(utils.NatsCA(), "\n", "\\n", -1),
		NatsCertificate:     strings.Replace(utils.NatsCertificate(), "\n", "\\n", -1),
		NatsPrivateKey:      strings.Replace(utils.NatsPrivateKey(), "\n", "\\n", -1),
	}
	settingsTmpl, err := template.ParseFiles(
		filepath.Join(utils.AgentDir(), "integration", "windows", "fixtures", "templates", "agent-configuration", "settings.json.tmpl"),
	)
	Expect(err).NotTo(HaveOccurred())

	outputFile, err := os.CreateTemp("", "agent-settings")
	defer outputFile.Close()
	Expect(err).NotTo(HaveOccurred())

	err = settingsTmpl.Execute(outputFile, agentSettings)
	outputFile.Close()

	command := exec.Command("scp", outputFile.Name(), fmt.Sprintf("%s:/bosh/agent-configuration/%s", utils.AgentIP(), filename))
	session, err := gexec.Start(command, ioutil.Discard, ioutil.Discard)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, 1*time.Minute).Should(gexec.Exit(0))
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
