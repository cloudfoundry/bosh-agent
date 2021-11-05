package windows_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/cloudfoundry/bosh-agent/agent/action"
	"github.com/cloudfoundry/bosh-agent/integration/utils"
	windowsutils "github.com/cloudfoundry/bosh-agent/integration/windows/utils"
	boshfileutil "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	agentGUID       = "123-456-789"
	agentID         = "agent." + agentGUID
	senderID        = "director.987-654-321"
	DefaultTimeout  = time.Minute
	DefaultInterval = time.Second
)

func getNetworkProperty(key string, natsClient *utils.NatsClient) string {
	message := fmt.Sprintf(`{"method":"get_state","arguments":["full"],"reply_to":"%s"}`, senderID)
	rawResponse, err := natsClient.SendRawMessage(message)
	Expect(err).NotTo(HaveOccurred())

	response := map[string]action.GetStateV1ApplySpec{}
	err = json.Unmarshal(rawResponse, &response)
	Expect(err).NotTo(HaveOccurred())

	for _, spec := range response["value"].NetworkSpecs {
		field, ok := spec.Fields[key]
		if !ok {
			return ""
		}
		if val, ok := field.(string); ok {
			return val
		}
	}
	return ""
}

var _ = Describe("An Agent running on Windows", func() {
	var (
		fs              boshsys.FileSystem
		natsClient      *utils.NatsClient
		blobstoreClient utils.BlobClient
	)

	BeforeEach(func() {
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\agent.json c:\\bosh\\agent.json")
		agent.RunPowershellCommand("cp c:\\bosh\\agent-configuration\\root-disk-settings.json c:\\bosh\\settings.json")
		agent.StartAgent()

		message := fmt.Sprintf(`{"method":"ping","arguments":[],"reply_to":"%s"}`, senderID)

		blobstoreClient = utils.NewBlobstore(windowsutils.BlobstoreURI())

		logger := boshlog.NewLogger(boshlog.LevelNone)
		cmdRunner := boshsys.NewExecCmdRunner(logger)
		fs = boshsys.NewOsFileSystem(logger)
		compressor := boshfileutil.NewTarballCompressor(cmdRunner, fs)

		natsClient = utils.NewNatsClient(compressor, blobstoreClient, windowsutils.FakeDirectorIP())
		err := natsClient.Setup()
		Expect(err).NotTo(HaveOccurred())

		testPing := func() (string, error) {
			response, err := natsClient.SendRawMessage(message)
			return string(response), err
		}

		Eventually(testPing, DefaultTimeout, DefaultInterval).Should(Equal(`{"value":"pong"}`))
	})

	AfterEach(func() {
		natsClient.Cleanup()

		agent.EnsureAgentServiceStopped()
	})

	It("responds to 'get_state' message over NATS", func() {
		getStateSpecAgentID := func() string {
			message := fmt.Sprintf(`{"method":"get_state","arguments":[],"reply_to":"%s"}`, senderID)
			rawResponse, err := natsClient.SendRawMessage(message)
			Expect(err).NotTo(HaveOccurred())

			response := map[string]action.GetStateV1ApplySpec{}
			err = json.Unmarshal(rawResponse, &response)
			Expect(err).NotTo(HaveOccurred())

			return response["value"].AgentID
		}

		Eventually(getStateSpecAgentID, DefaultTimeout, DefaultInterval).Should(Equal(agentGUID))
	})

	It("cleans up SSH users after session exits", func() {
		time := strconv.Itoa(int(time.Now().Unix()))
		sshTestUser := fmt.Sprintf("bosh_testuser_%s", time[len(time)-5:])
		setupResult, sshClientConfig, err := natsClient.SetupSSH(sshTestUser, senderID)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupResult.Status).To(Equal("success"))

		output := agent.RunPowershellCommand(`get-wmiobject -class win32_userprofile | Where { $_.LocalPath -eq 'C:\Users\%s'}`, sshTestUser)
		Expect(output).To(MatchRegexp(sshTestUser))

		tunnelClient, err := windowsutils.GetSSHTunnelClient()
		Expect(err).NotTo(HaveOccurred())

		agentSSHAddress := fmt.Sprintf("%s:22", windowsutils.AgentIP())
		proxyConnection, err := tunnelClient.Dial("tcp", agentSSHAddress)
		Expect(err).NotTo(HaveOccurred())

		proxyClientConnection, proxyClientChannel, proxyClientRequest, err := ssh.NewClientConn(proxyConnection, agentSSHAddress, sshClientConfig)
		Expect(err).NotTo(HaveOccurred())

		client := ssh.NewClient(proxyClientConnection, proxyClientChannel, proxyClientRequest)

		Expect(err).NotTo(HaveOccurred())

		session, err := client.NewSession()
		Expect(err).NotTo(HaveOccurred())

		err = session.Close()
		Expect(err).NotTo(HaveOccurred())

		cleanupResult, err := natsClient.CleanupSSH(sshTestUser, senderID)
		Expect(err).NotTo(HaveOccurred())

		Expect(cleanupResult.Status).To(Equal("success"))

		_, _, exitCode, _ := agent.RunPowershellCommandWithResponses(`NET.exe USER %s`, sshTestUser)
		Expect(exitCode).To(Equal(1))

		deletableUserProfileContent := agent.RunPowershellCommand(`Get-ChildItem -force -recurse -attributes !Directory -Exclude 'ntuser.dat*' , 'usrclass.dat*' /users/%s`, sshTestUser)
		Expect(deletableUserProfileContent).To(BeEmpty())
	})

	It("includes memory vitals in the 'get_state' response", func() {
		message := fmt.Sprintf(`{"method":"get_state","arguments":["full"],"reply_to":"%s"}`, senderID)

		checkVitals := func() error {
			b, err := natsClient.SendRawMessage(message)
			if err != nil {
				return err
			}

			var res map[string]action.GetStateV1ApplySpec
			if err := json.Unmarshal(b, &res); err != nil {
				return err
			}

			v := res["value"].Vitals
			if v == nil {
				return fmt.Errorf("nil Vitals")
			}
			if v.Mem.Kb == "" || v.Mem.Percent == "" {
				return fmt.Errorf("Empty Memory Vitals: %+v", v.Mem)
			}
			return nil
		}

		Eventually(checkVitals, DefaultTimeout, DefaultInterval).Should(Succeed())
	})

	It("can run a run_errand action", func() {
		natsClient.PrepareJob("say-hello")

		runErrandResponse, err := natsClient.RunErrand()
		Expect(err).NotTo(HaveOccurred())

		runErrandCheck := natsClient.CheckErrandResultStatus(runErrandResponse["value"]["agent_task_id"])
		Eventually(runErrandCheck, DefaultTimeout, DefaultInterval).Should(Equal(action.ErrandResult{
			Stdout:     "hello world\r\n",
			ExitStatus: 0,
		}))
	})

	It("can start a job", func() {
		natsClient.PrepareJob("say-hello")

		runStartResponse, err := natsClient.RunStart()
		Expect(err).NotTo(HaveOccurred())
		Expect(runStartResponse["value"]).To(Equal("started"))

		agentState := natsClient.GetState()
		Expect(agentState.JobState).To(Equal("running"))
	})

	It("can run a drain script", func() {
		natsClient.PrepareJob("say-hello")

		err := natsClient.RunDrain()
		Expect(err).NotTo(HaveOccurred())

		logsDir, err := fs.TempDir("windows-agent-drain-test")
		Expect(err).NotTo(HaveOccurred())
		defer fs.RemoveAll(logsDir)

		natsClient.FetchLogs(logsDir)

		drainLogContents, err := fs.ReadFileString(filepath.Join(logsDir, "say-hello", "drain.log"))
		Expect(err).NotTo(HaveOccurred())

		Expect(drainLogContents).To(ContainSubstring("Hello from drain"))
	})

	It("can unmonitor the job during drain script", func() {
		natsClient.PrepareJob("unmonitor-hello")

		runStartResponse, err := natsClient.RunStart()
		Expect(err).NotTo(HaveOccurred())
		Expect(runStartResponse["value"]).To(Equal("started"))

		agentState := natsClient.GetState()
		Expect(agentState.JobState).To(Equal("running"))

		err = natsClient.RunDrain()
		Expect(err).NotTo(HaveOccurred())

		logsDir, err := fs.TempDir("windows-agent-drain-test")
		Expect(err).NotTo(HaveOccurred())
		defer fs.RemoveAll(logsDir)

		natsClient.FetchLogs(logsDir)

		drainLogContents, err := fs.ReadFileString(filepath.Join(logsDir, "unmonitor-hello", "drain.log"))
		Expect(err).NotTo(HaveOccurred())

		Expect(drainLogContents).To(ContainSubstring("success"))
	})

	It("stops alerting failing jobs when job is stopped", func() {
		natsClient.PrepareJob("crashes-on-start")
		runStartResponse, err := natsClient.RunStart()
		Expect(err).NotTo(HaveOccurred())
		Expect(runStartResponse["value"]).To(Equal("started"))

		Eventually(func() string {
			return natsClient.GetState().JobState
		}, DefaultTimeout, DefaultInterval).Should(Equal("failing"))

		Eventually(func() (string, error) {
			alert, err := natsClient.GetNextAlert(10 * time.Second)
			if err != nil {
				return "", err
			}
			return alert.Title, nil
		}).Should(MatchRegexp(`crash-service \(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\) - pid failed - Start`))

		err = natsClient.RunStop()
		Expect(err).NotTo(HaveOccurred())

		_, err = natsClient.GetNextAlert(10 * time.Second)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("nats: timeout"))
	})

	It("can run arbitrary user scripts", func() {
		natsClient.PrepareJob("say-hello")

		err := natsClient.RunScript("pre-start")
		Expect(err).NotTo(HaveOccurred())

		logsDir, err := fs.TempDir("windows-agent-prestart-test")
		Expect(err).NotTo(HaveOccurred())
		defer fs.RemoveAll(logsDir)

		natsClient.FetchLogs(logsDir)

		prestartStdoutContents, err := fs.ReadFileString(filepath.Join(logsDir, "say-hello", "pre-start.stdout.log"))
		Expect(err).NotTo(HaveOccurred())

		Expect(prestartStdoutContents).To(ContainSubstring("Hello from stdout"))

		prestartStderrContents, err := fs.ReadFileString(filepath.Join(logsDir, "say-hello", "pre-start.stderr.log"))
		Expect(err).NotTo(HaveOccurred())

		Expect(prestartStderrContents).To(ContainSubstring("Hello from stderr"))
	})

	It("can compile packages", func() {
		const (
			blobName     = "blob.tar"
			fileName     = "output.txt"
			fileContents = "i'm a compiled package!"
		)
		result, err := natsClient.CompilePackage("simple-package")
		Expect(err).NotTo(HaveOccurred())

		tempDir, err := fs.TempDir("windows-agent-compile-test")
		Expect(err).NotTo(HaveOccurred())

		path := filepath.Join(tempDir, blobName)
		Expect(blobstoreClient.Get(result.BlobstoreID, path)).To(Succeed())

		tarPath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(tarPath)

		err = exec.Command("tar", "xf", path, "-C", tarPath).Run()
		Expect(err).NotTo(HaveOccurred())

		out, err := ioutil.ReadFile(filepath.Join(tarPath, fileName))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(ContainSubstring(fileContents))
	})

	It("Includes the default IP in the 'get_state' response", func() {
		getNetwork := func(key string) string {
			return getNetworkProperty(key, natsClient)
		}
		Eventually(getNetwork("ip"), DefaultTimeout, DefaultInterval).ShouldNot(BeEmpty())
		Eventually(getNetwork("gateway"), DefaultTimeout, DefaultInterval).ShouldNot(BeEmpty())
		Eventually(getNetwork("netmask"), DefaultTimeout, DefaultInterval).ShouldNot(BeEmpty())
	})

	It("can compile longpath complex package", func() {
		_, err := natsClient.CompilePackage("longpath-package")
		Expect(err).NotTo(HaveOccurred())
	})

	It("can cleanup package compilation dependencies when they are initially still in use", func() {
		blobref, err := natsClient.CompilePackage("go")
		Expect(err).NotTo(HaveOccurred())

		_, err = natsClient.CompilePackageWithDeps(
			"execution-lock",
			map[string]utils.MarshalableBlobRef{"go": *blobref},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when there are multiple networks", func() {
		BeforeEach(func() {
			natsClient.PrepareJob("add-network-interface")

			err := natsClient.RunScript("run")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			natsClient.PrepareJob("remove-network-interface")

			err := natsClient.RunScript("run")
			Expect(err).NotTo(HaveOccurred())
		})

		It("chooses correct IP for default network", func() {
			getNetwork := func(key string) string {
				return getNetworkProperty(key, natsClient)
			}
			Eventually(getNetwork("ip"), DefaultTimeout, DefaultInterval).ShouldNot(BeEmpty())
			Expect(getNetwork("ip")).ToNot(HavePrefix("172."))
		})
	})
})
