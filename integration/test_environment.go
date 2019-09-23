package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"time"

	"github.com/cloudfoundry/bosh-agent/agentclient"
	"github.com/cloudfoundry/bosh-agent/integration/integrationagentclient"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	"github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
)

const agentCert = `-----BEGIN CERTIFICATE-----
MIIC3zCCAcegAwIBAgIBADANBgkqhkiG9w0BAQUFADAzMQswCQYDVQQGEwJVUzEQ
MA4GA1UECgwHUGl2b3RhbDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTEzMTIwMTIy
MTEzMloXDTE2MTIwMTIyMTEzMlowMzELMAkGA1UEBhMCVVMxEDAOBgNVBAoMB1Bp
dm90YWwxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBALra3YIx0O6PLcQsHAXbFzJye1M/Am3gCMcX5BTui+I7vfyMXG4w
FyXHLf9lWwe23ypvP4c1+LNTq7UTTZuidrnadlADgdDned0C09bOKv5Nzk466XTR
fNyBEyxaZzHQLa9SmDLgq1GUun8oEsxZ+uXhMq6kikRu5LBCChtVCW6LzG/FE1qm
jSSH6iaOwk2yQxKVqUKNPfz1PqtRgaUBjVWrh2+Wf22KzOTORouBOrSfdxep1Cjz
lQWt2W5l05dvf2vZTlaqDCk8PBF36FWPlwmZxRsHGACVuckl3yJ69jIaa+i+mK3k
cfi05ZafWeFwm21ahqzwK/kGsK1ofPHKxE8CAwEAATANBgkqhkiG9w0BAQUFAAOC
AQEAD1VzwtWCx32pQi5l0oFfjWqktnqfhs/Rr0ZpwacBBXHBvKuz9ENXmblt5pZu
JP7je+uXQD+da/oVhl0US2L0upIdMmD1utVXYHfRji5r/tIPl2SEKTrFiNZR1Wp6
J0nE/BW7nm41dXRBIAZR71yproaQrt1tFDFZvdfhwHGLC51L6toOhk/7S604sxbk
qV0tzT+VaR4hh09FEt9xGmB/3yFh329Yib8ScT94nKzSzoNoDp4Ms/smFhF4lUio
7SD2+b2/nt8Mcz7q58nYvZteipRrmkOFszlNF5dU31FjvRLITn0bhiOOFRD3qAou
cSOr1qwsAKuu6MzYNh2ubsLvfg==
-----END CERTIFICATE-----`

const agentKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAutrdgjHQ7o8txCwcBdsXMnJ7Uz8CbeAIxxfkFO6L4ju9/Ixc
bjAXJcct/2VbB7bfKm8/hzX4s1OrtRNNm6J2udp2UAOB0Od53QLT1s4q/k3OTjrp
dNF83IETLFpnMdAtr1KYMuCrUZS6fygSzFn65eEyrqSKRG7ksEIKG1UJbovMb8UT
WqaNJIfqJo7CTbJDEpWpQo09/PU+q1GBpQGNVauHb5Z/bYrM5M5Gi4E6tJ93F6nU
KPOVBa3ZbmXTl29/a9lOVqoMKTw8EXfoVY+XCZnFGwcYAJW5ySXfInr2Mhpr6L6Y
reRx+LTllp9Z4XCbbVqGrPAr+QawrWh88crETwIDAQABAoIBAFh3DrB5TWXku9JI
3+uV0uG7ec/r4QaMLxuzn/SZC/lMN6K+AXTZp9vm9UwZfIOmfPnmObmWP+0HDCBq
xy+MN5G+cI1pW6jALt4IXKsyaQCFbctz8Nux4t+y7JTvKDRZT4fWHuDXpcS2GaXi
HyRI5ZS4jfpJRH0p03PvkEFofVKsZfLAlwnx7HhPqztyc6huu7njjP6XUYbvrhOS
otE/wyyLyT36rQvUVLHfrco2Ib1r58sWb1hL84/5HnbxwsEelBAAGL5VGu2f0zLP
qxmy4XWJg9U/UKzG9gXmbcpDUn1pUcZV1rSSJ/wfbdvBSLF+dQaE6QdjGeAyrt7M
I7gCViECgYEA5ui1aXomHIwqbzll/qotebYNL8AOcpgRJTtzODcnzoe/5aA7yVox
vOFTD9rODaVRhtdC/ZHsU6rID+tx9gQdjLUEXUqS3KsPXSiwbYBPXnEtpgWz/RAu
qpxpMn3kfVbQgJjkfksWvlHNzLj5q4ZhZyL/WV3P9DON8vuP/hwwQLMCgYEAzyiv
rXx/ERnEQOpXEvPtCj04QUGCyW2o4nYIrN3OfO3PM8PnNfB8Z2LKDyuzX9qvWghg
z81KSNByW4OLdvrnX5natfxdu/6qAl18N+z/DFXuTgXi8nG9/2R5c3Gn5CQ5yab7
S4G4Pt7YwfoiqFv9j2b0DE3e5UVicGQIuWmaQ/UCgYBf61swraUXRsJ54YYU++xY
Gt/vbgCSacj1hnSebYsDqDB22tD7G5R9ubwfYe0mjf4H3XPekbdyKgdhVZTJdXww
7yEY/9lyAT0onbZsRliyCqHDzaqu/QHlrYOljdZUmrOSN/Dy5Y5VEPZEjLJqJjBf
/5HDNc0kzCWzQfzWui7xMQKBgGNA8ysEAz2GQul3XdDO3juRqWpaoPcxe0FFnFJ2
04A30JbUveqyFmjShE1QetjqRim06e2mRnksph4CoMeY31KGvKuFBsQT+BC6CdIh
0vFuGod3eoz+wjGjSi1tvysn0Cg1wSEkPcqhqukFl6VirdIPWc6rYKgo3klLJILx
feAhAoGABC0apuKQD2IZZXZtDuUI9I4AemPPh0yKvFfTJxmxQ0fTlWjqdcG5nYdh
tSMBlZwsd6DRlK7dWJ/WHZXuXNeOX6ehSQFmql5/XPNd7INa5My6DDPZr1chh0WJ
QgK94NXJDoDd1OZjpUBMPLVa8d20/RdGNW8OMolJpzEPhg0r7Ac=
-----END RSA PRIVATE KEY-----`

type TestEnvironment struct {
	cmdRunner        boshsys.CmdRunner
	currentDeviceNum int
	sshTunnelProc    boshsys.Process
	logger           logger.Logger
	agentClient      agentclient.AgentClient
	deviceMap        map[int]string
	sshClient        *ssh.Client
}

func NewTestEnvironment(
	cmdRunner boshsys.CmdRunner,
) (*TestEnvironment, error) {
	client, err := dialSSHClient(cmdRunner)
	if err != nil {
		return nil, err
	}

	return &TestEnvironment{
		cmdRunner:        cmdRunner,
		currentDeviceNum: 2,
		logger:           logger.NewLogger(logger.LevelDebug),
		deviceMap:        make(map[int]string),
		sshClient:        client,
	}, nil
}

func (t *TestEnvironment) SetupConfigDrive() error {
	deviceNum, err := t.AttachLoopDevice(10)
	if err != nil {
		return err
	}

	setupConfigDriveTemplate := `
sudo mkfs -t ext3 -m 1 -v %s
sudo e2label %s config-2
sudo udevadm settle
sudo rm -rf /tmp/config-drive
sudo mkdir /tmp/config-drive
sudo mount /dev/disk/by-label/config-2 /tmp/config-drive
sudo chown vagrant:vagrant /tmp/config-drive
sudo mkdir -p /tmp/config-drive/ec2/latest
sudo cp %s/meta-data.json /tmp/config-drive/ec2/latest/meta-data.json
sudo cp %s/user-data.json /tmp/config-drive/ec2/latest/user-data.json
sudo umount /tmp/config-drive
`
	setupConfigDriveScript := fmt.Sprintf(setupConfigDriveTemplate, t.deviceMap[deviceNum], t.deviceMap[deviceNum], t.assetsDir(), t.assetsDir())

	_, err = t.RunCommand(setupConfigDriveScript)
	return err
}

type byLen []string

func (a byLen) Len() int           { return len(a) }
func (a byLen) Less(i, j int) bool { return len(a[i]) > len(a[j]) }
func (a byLen) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (t *TestEnvironment) DetachDevice(dir string) error {
	mountPoints, err := t.RunCommand(fmt.Sprintf(`sudo mount | grep "on %s" | cut -d ' ' -f 3`, dir))
	if err != nil {
		return err
	}

	mountPointsSlice := strings.Split(mountPoints, "\n")
	sort.Sort(byLen(mountPointsSlice))
	for _, mountPoint := range mountPointsSlice {
		if mountPoint != "" {
			t.RunCommand(fmt.Sprintf("sudo fuser -km %s", mountPoint))
			t.RunCommand(fmt.Sprintf("sudo umount %s", mountPoint))
		}
	}

	_, err = t.RunCommand(fmt.Sprintf("sudo rm -rf %s", dir))
	return err
}

func (t *TestEnvironment) CleanupDataDir() error {
	t.RunCommand(`sudo /var/vcap/bosh/bin/monit stop all`)

	_, err := t.RunCommand("! mount | grep -q ' on /tmp ' || sudo umount /tmp")
	if err != nil {
		return err
	}

	err = t.DetachDevice("/var/tmp")
	if err != nil {
		return err
	}

	err = t.DetachDevice("/var/log")
	if err != nil {
		return err
	}

	err = t.DetachDevice("/var/vcap/data")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo mkdir -p /var/tmp")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chmod 700 /var/tmp")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chmod 1777 /tmp")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo mkdir -p /var/log")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chmod 775 /var/log")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chown root:syslog /var/log")
	if err != nil {
		return err
	}

	return nil
}

func (t *TestEnvironment) ResetDeviceMap() error {
	for n, loopDevice := range t.deviceMap {
		t.DetachLoopDevice(loopDevice)
		_, err := t.RunCommand(fmt.Sprintf("sudo rm -f %s", fmt.Sprintf("/virtualfs-%d", n)))
		if err != nil {
			return err
		}
		t.deviceMap = make(map[int]string)
	}
	return nil
}

// ConfigureAgentForGenericInfrastructure executes the agent_runit.sh asset.
// Required for reverse-compatibility with older bosh-lite
// (remove once a new warden stemcell is built).
func (t *TestEnvironment) ConfigureAgentForGenericInfrastructure() error {
	_, err := t.RunCommand(
		fmt.Sprintf(
			"sudo cp %s/agent_runit.sh /etc/service/agent/run",
			t.assetsDir(),
		),
	)
	return err
}

func (t *TestEnvironment) CleanupLogFile() error {
	_, err := t.RunCommand("sudo truncate -s 0 /var/vcap/bosh/log/current")
	return err
}

func (t *TestEnvironment) CleanupSSH() error {
	_, err := t.RunCommand("sudo rm -rf /var/vcap/bosh_ssh")
	return err
}

func (t *TestEnvironment) LogFileContains(content string) bool {
	_, err := t.RunCommand(fmt.Sprintf(`sudo grep "%s" /var/vcap/bosh/log/current`, content))
	return err == nil
}

func (t *TestEnvironment) AttachDevice(devicePath string, partitionSize, numPartitions int) error {
	partitionPath := devicePath
	for i := 0; i <= numPartitions; i++ {
		if i > 0 {
			partitionPath = fmt.Sprintf("%s%d", devicePath, i)
		}

		deviceNum, err := t.AttachLoopDevice(partitionSize)
		if err != nil {
			return err
		}

		c := fmt.Sprintf("ls -al %s | cut -d' ' -f 6", t.deviceMap[deviceNum])
		output, err := t.RunCommand(c)
		minorNum := strings.TrimSpace(output)
		if err != nil {
			return err
		}

		err = t.RemoveDevice(partitionPath)
		if err != nil {
			return err
		}

		c = fmt.Sprintf("sudo mknod %s b 7 %s", partitionPath, minorNum)
		_, err = t.RunCommand(c)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TestEnvironment) AttachPartitionedRootDevice(devicePath string, sizeInMB, rootPartitionSizeInMB int) (string, error) {
	err := t.AttachDevice(devicePath, sizeInMB, 3)
	if err != nil {
		return "", err
	}

	// Create only first partition, agent will partition the rest for ephemeral disk
	partitionTemplate := `
echo '1,%d,L,' | sudo sfdisk -uS %s
`

	partitionScript := fmt.Sprintf(partitionTemplate, rootPartitionSizeInMB*2048, devicePath)
	_, err = t.RunCommand(partitionScript)
	if err != nil {
		return "", err
	}

	rootLink, err := t.RunCommand("df / | grep /dev/ | cut -d' ' -f1")
	if err != nil {
		return "", err
	}

	oldRootDevice, err := t.RunCommand(fmt.Sprintf("readlink -f %s", rootLink))
	if err != nil {
		return "", err
	}

	_, err = t.RunCommand(fmt.Sprintf("sudo mv %s %s-temp", strings.TrimSpace(oldRootDevice), strings.TrimSpace(oldRootDevice)))
	if err != nil {
		return "", err
	}

	// Agent reads the symlink to get root device
	// Create a symlink to our fake device
	_, err = t.RunCommand(fmt.Sprintf("sudo ln -sf %s1 %s", devicePath, strings.TrimSpace(rootLink)))

	if err != nil {
		return strings.TrimSpace(oldRootDevice), err
	}

	return strings.TrimSpace(oldRootDevice), nil
}

func (t *TestEnvironment) DetachPartitionedRootDevice(rootLink string, devicePath string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo rm -f %s", rootLink))
	if err != nil {
		return err
	}

	partitionPath := devicePath
	for i := 3; i >= 0; i-- {
		if i > 0 {
			partitionPath = fmt.Sprintf("%s%d", devicePath, i)
		}

		if _, err := t.RunCommand(fmt.Sprintf("losetup %s", partitionPath)); err == nil {
			if output, _ := t.RunCommand(fmt.Sprintf("sudo mount | grep '%s ' | awk '{print $3}'", partitionPath)); output != "" {
				t.RunCommand(fmt.Sprintf("sudo umount -l %s", output))
			}

			if i > 0 {
				_, _ = t.RunCommand(fmt.Sprintf("sudo parted %s rm %d", devicePath, i))
			}

			err = t.DetachLoopDevice(partitionPath)
			if err != nil {
				return err
			}

			err = t.RemoveDevice(partitionPath)
			if err != nil {
				return err
			}
		}

	}

	_, err = t.RunCommand(fmt.Sprintf("sudo mv %s-temp %s", rootLink, rootLink))
	if err != nil {
		return err
	}

	return nil
}

func (t *TestEnvironment) RemoveDevice(devicePath string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo rm -f %s", devicePath))
	return err
}

func (t *TestEnvironment) AttachLoopDevice(size int) (int, error) {
	deviceNum := t.currentDeviceNum

	output, err := t.RunCommand("sudo losetup -f")
	devicePath := strings.TrimSpace(output)
	if err != nil {
		return 0, err
	}

	if oldDevicePath, ok := t.deviceMap[deviceNum]; ok {
		err := t.DetachLoopDevice(oldDevicePath)
		if err != nil {
			return 0, err
		}
	}

	attachDeviceTemplate := `
sudo rm -rf /virtualfs-%d
sudo dd if=/dev/zero of=/virtualfs-%d bs=1M count=%d
sudo losetup %s /virtualfs-%d
`
	attachDeviceScript := fmt.Sprintf(attachDeviceTemplate, deviceNum, deviceNum, size, devicePath, deviceNum)
	_, err = t.RunCommand(attachDeviceScript)
	if err != nil {
		return 0, err
	}

	t.deviceMap[deviceNum] = devicePath
	t.currentDeviceNum++

	return deviceNum, nil
}

func (t *TestEnvironment) DetachLoopDevice(devicePath string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo losetup -d %s", devicePath))
	return err
}

func (t *TestEnvironment) DetachLoopDevices() error {
	_, err := t.RunCommand(fmt.Sprintf("sudo losetup -D"))
	return err
}

func (t *TestEnvironment) SetUpDummyNetworkInterface(ip, mac string) error {
	return t.RunCommandChain(
		"sudo modprobe dummy",
		"sudo ip link set name dummy0 dev dummy0",
		fmt.Sprintf("sudo ip link set dev dummy0 address %s", mac),
		"sudo ip link set dev dummy0 arp on",
		fmt.Sprintf("sudo ip addr add %s dev dummy0", ip),
		fmt.Sprintf("sudo ip neigh add to %s lladdr %s dev dummy0 nud reachable", ip, mac),
	)
}

func (t *TestEnvironment) RunCommandChain(commands ...string) error {
	for _, command := range commands {
		_, err := t.RunCommand(command)
		if err != nil {
			return errors.WrapErrorf(err, "Error running %s", command)
		}
	}
	return nil
}

func (t *TestEnvironment) TearDownDummyNetworkInterface() error {
	_, err := t.RunCommand("sudo rmmod dummy")
	return err
}

func (t *TestEnvironment) UpdateAgentConfig(configFile string) error {
	_, err := t.RunCommand(
		fmt.Sprintf(
			"sudo cp %s/%s /var/vcap/bosh/agent.json",
			t.assetsDir(),
			configFile,
		),
	)
	return err
}

func (t *TestEnvironment) RestartAgent() error {
	err := t.StopAgent()
	if err != nil {
		return err
	}

	return t.StartAgent()
}

func (t *TestEnvironment) StopAgent() error {
	_, err := t.RunCommand("nohup sudo sv stop agent &")
	return err
}

func (t *TestEnvironment) StartAgent() error {
	_, err := t.RunCommand("nohup sudo sv start agent &")
	return err
}

type emptyReader struct{}

func (er emptyReader) Read(p []byte) (int, error) {
	time.Sleep(1 * time.Second)
	return 0, nil
}

func (t *TestEnvironment) StartAgentTunnel(mbusUser, mbusPass string, mbusPort int) (*integrationagentclient.IntegrationAgentClient, error) {
	if t.sshTunnelProc != nil {
		return nil, fmt.Errorf("Already running")
	}

	sshCmd := boshsys.Command{
		Name: "vagrant",
		Args: []string{
			"ssh",
			"--",
			fmt.Sprintf("-L16868:127.0.0.1:%d", mbusPort),
		},
		Stdin: emptyReader{},
	}
	newTunnelProc, err := t.cmdRunner.RunComplexCommandAsync(sshCmd)
	if err != nil {
		return nil, err
	}
	t.sshTunnelProc = newTunnelProc

	httpClient := httpclient.NewHTTPClient(httpclient.DefaultClient, t.logger)
	mbusURL := fmt.Sprintf("https://%s:%s@localhost:16868", mbusUser, mbusPass)
	client := integrationagentclient.NewIntegrationAgentClient(mbusURL, "fake-director-uuid", 1*time.Second, 10, httpClient, t.logger)

	for i := 1; i < 90; i++ {
		t.logger.Debug("test environment", "Trying to contact agent via ssh tunnel...")
		time.Sleep(1 * time.Second)
		_, err := client.Ping()
		if err == nil {
			break
		}
		t.logger.Debug("test environment", err.Error())
	}
	return client, nil
}

func (t *TestEnvironment) StopAgentTunnel() error {
	if t.sshTunnelProc == nil {
		return fmt.Errorf("Not running")
	}
	t.sshTunnelProc.Wait()
	t.sshTunnelProc = nil
	return nil
}

func (t *TestEnvironment) StartRegistry(settings boshsettings.Settings) error {
	emptyCert := boshsettings.CertKeyPair{}
	if settings.Env.Bosh.Mbus.Cert == emptyCert {
		settings.Env.Bosh.Mbus.Cert.Certificate = agentCert
		settings.Env.Bosh.Mbus.Cert.PrivateKey = agentKey
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo rm -f /var/vcap/bosh/settings.json")
	if err != nil {
		return err
	}

	t.RunCommand("sudo killall -9 fake-registry")

	_, err = t.RunCommand(
		fmt.Sprintf(
			`nohup %s/tmp/fake-registry -user user -password pass -host 127.0.0.1 -port 9090 -instance instance-id -settings %s &> /dev/null &`,
			t.agentDir(),
			strconv.Quote(string(settingsJSON)),
		),
	)
	return err
}

func (t *TestEnvironment) GetVMNetworks() (boshsettings.Networks, error) {
	stdout, _, _, err := t.cmdRunner.RunCommand("vagrant", "status")
	if err != nil {
		return boshsettings.Networks{}, err
	}

	if strings.Contains(stdout, "virtualbox") {
		return boshsettings.Networks{
			"eth0": {
				Type: "dynamic",
			},
			"eth1": {
				Type:    "manual",
				IP:      "192.168.50.4",
				Netmask: "255.255.255.0",
			},
		}, nil
	}

	if strings.Contains(stdout, "aws") {
		return boshsettings.Networks{
			"eth0": {
				Type: "dynamic",
			},
		}, nil
	}

	return boshsettings.Networks{}, nil
}

func (t *TestEnvironment) GetFileContents(filePath string) (string, error) {
	return t.RunCommand(
		fmt.Sprintf(
			`cat %s`,
			filePath,
		),
	)
}

func (t *TestEnvironment) RunCommand(command string) (string, error) {
	s, err := t.sshClient.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()
	out, err := s.Output(command)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (t *TestEnvironment) RunCommand3(command string) (string, string, int, error) {
	return t.cmdRunner.RunCommand("vagrant", "ssh", "--", command)
}

func (t *TestEnvironment) CreateSensitiveBlobFromAsset(assetPath, blobID string) error {
	_, err := t.RunCommand("sudo mkdir -p /var/vcap/data/sensitive_blobs")
	if err != nil {
		return err
	}

	_, _, _, err = t.cmdRunner.RunCommand(
		"vagrant",
		"ssh",
		"--",
		fmt.Sprintf("sudo cp %s/%s /var/vcap/data/sensitive_blobs/%s", t.assetsDir(), assetPath, blobID),
	)

	return err
}

func (t *TestEnvironment) CreateBlobFromAsset(assetPath, blobID string) error {
	_, err := t.RunCommand("sudo mkdir -p /var/vcap/data/blobs")
	if err != nil {
		return err
	}

	_, _, _, err = t.cmdRunner.RunCommand(
		"vagrant",
		"ssh",
		"--",
		fmt.Sprintf("sudo cp %s/%s /var/vcap/data/blobs/%s", t.assetsDir(), assetPath, blobID),
	)

	return err
}

func (t *TestEnvironment) CreateBlobFromAssetInActualBlobstore(assetPath, blobstorePath, blobID string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo mkdir -p %s", blobstorePath))
	if err != nil {
		return err
	}

	_, _, _, err = t.cmdRunner.RunCommand(
		"vagrant",
		"ssh",
		"--",
		fmt.Sprintf("sudo cp %s %s", filepath.Join(t.assetsDir(), assetPath), filepath.Join(blobstorePath, blobID)),
	)

	return err
}

func (t *TestEnvironment) CreateBlobFromStringInActualBlobstore(contents, blobstorePath, blobID string) (string, error) {
	_, err := t.RunCommand(fmt.Sprintf("sudo mkdir -p %s", blobstorePath))
	if err != nil {
		return "", err
	}

	remoteBlobPath := filepath.Join(blobstorePath, blobID)
	_, _, _, err = t.cmdRunner.RunCommandWithInput(
		contents,
		"vagrant",
		"ssh",
		"--",
		fmt.Sprintf("cat | sudo tee %s", remoteBlobPath),
	)
	if err != nil {
		return "", err
	}

	blobDigest, _, _, err := t.cmdRunner.RunCommand(
		"vagrant",
		"ssh",
		"--",
		fmt.Sprintf("sudo shasum %s | cut -f 1 -d ' '", remoteBlobPath),
	)

	return blobDigest, err
}

func (t *TestEnvironment) agentDir() string {
	return "/home/vagrant/go/src/github.com/cloudfoundry/bosh-agent"
}

func (t *TestEnvironment) assetsDir() string {
	return fmt.Sprintf("%s/integration/assets", t.agentDir())
}

func dialSSHClient(cmdRunner boshsys.CmdRunner) (*ssh.Client, error) {
	stdout, _, _, err := cmdRunner.RunCommand("vagrant", "ssh-config")
	if err != nil {
		return nil, err
	}

	config, err := ssh_config.Decode(strings.NewReader(stdout))
	if err != nil {
		return nil, err
	}
	user, err := config.Get("default", "User")
	if err != nil {
		return nil, err
	}
	addr, err := config.Get("default", "HostName")
	if err != nil {
		return nil, err
	}
	port, err := config.Get("default", "Port")
	if err != nil {
		return nil, err
	}
	keyPath, err := config.Get("default", "IdentityFile")
	if err != nil {
		return nil, err
	}
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return ssh.Dial("tcp", fmt.Sprintf("%s:%s", addr, port), &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
}
