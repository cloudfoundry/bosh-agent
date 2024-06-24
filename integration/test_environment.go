package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"

	"github.com/cloudfoundry/bosh-agent/integration/integrationagentclient"
	"github.com/cloudfoundry/bosh-agent/settings"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
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
	writerPrinter    writerPrinter
	deviceMap        map[int]string
	sshClient        *ssh.Client
	AgentClient      *integrationagentclient.IntegrationAgentClient
	AgentSettings    settings.Settings
	mbusUser         string
	mbusPass         string
	mbusPort         int
}

type writerPrinter interface {
	io.Writer

	Print(a ...interface{})
	Printf(format string, a ...interface{})
	Println(a ...interface{})
}

func NewTestEnvironment(cmdRunner boshsys.CmdRunner, wp writerPrinter) (*TestEnvironment, error) {
	client, err := dialSSHClient(cmdRunner)
	if err != nil {
		return nil, err
	}

	return &TestEnvironment{
		cmdRunner:        cmdRunner,
		currentDeviceNum: 2,
		writerPrinter:    wp,
		deviceMap:        make(map[int]string),
		sshClient:        client,
		mbusUser:         "mbus-user",
		mbusPass:         "mbus-pass",
		mbusPort:         6868,
	}, nil
}

type byLen []string

func (a byLen) Len() int           { return len(a) }
func (a byLen) Less(i, j int) bool { return len(a[i]) > len(a[j]) }
func (a byLen) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func (t *TestEnvironment) DetachDevice(dir string) error {
	mountPoints, err := t.RunCommand(fmt.Sprintf(`sudo mount | grep "on %s" | cut -d ' ' -f 3`, dir))
	if err != nil {
		t.writerPrinter.Printf("DetachDevice: %s, Msg: %s", err, mountPoints)
		return err
	}

	mountPointsSlice := strings.Split(mountPoints, "\n")
	sort.Sort(byLen(mountPointsSlice))
	for _, mountPoint := range mountPointsSlice {
		if mountPoint != "" {
			out, ignoredErr := t.RunCommand(fmt.Sprintf("sudo fuser -km %s", mountPoint))
			// running fuser -k also kills the ssh session, this will always produce an error
			// only print the error if out is not empty.
			if ignoredErr != nil && out != "" {
				t.writerPrinter.Printf("DetachDevice: %s, Msg: %s", ignoredErr, out)
			}

			// Lazily unmount /var/log to prevent intermittent test failures. As of 2024-06-24, this mount point
			// is a bind mount of /var/vcap/data/root_log. For reasons we don't currently understand the
			//'fuser -k' doesn't seem to consistently terminate processes in time to do the umount, but this is
			// the only mount that has this problem.
			//
			// Because we later unmount /var/vcap/data, lazily unmounting /var/log will eventually alert us if
			// anyone has handles open in that mount point... so we'll eventually fail loudly, making this not
			// a catastrophically bad thing to do.
			if mountPoint == "/var/log" {
				_, ignoredErr = t.RunCommand(fmt.Sprintf("sudo umount --lazy %s", mountPoint))
			} else {
				_, ignoredErr = t.RunCommand(fmt.Sprintf("sudo umount %s", mountPoint))
			}
			if ignoredErr != nil {
				t.writerPrinter.Printf("DetachDevice: %s", ignoredErr)
			}
		}
	}

	_, err = t.RunCommand(fmt.Sprintf("sudo rm -rf %s", dir))
	return err
}

func (t *TestEnvironment) CleanupDataDir() error {
	_, ignoredErr := t.RunCommand("sudo /var/vcap/bosh/bin/monit stop all")
	if ignoredErr != nil {
		t.writerPrinter.Printf("CleanupDataDir: %s", ignoredErr)
	}

	err := t.ensureMonitStopped()
	if err != nil {
		return err
	}

	_, err = t.RunCommand("! mount | grep -q ' on /tmp ' || sudo umount /tmp")
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

	err = t.DetachDevice("/opt")
	if err != nil {
		return err
	}

	err = t.DetachDevice("/var/opt")
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

	_, err = t.RunCommand("sudo mkdir -p /var/opt")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chmod 775 /var/opt")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chown root:root /var/opt")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo mkdir -p /opt")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chmod 775 /opt")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("sudo chown root:root /opt")
	if err != nil {
		return err
	}

	return nil
}

func (t *TestEnvironment) ensureMonitStopped() error {
	monitStopped := false
	begin := time.Now()
	for i := 0; i < 3; i++ {
		monitProcessList, err := t.RunCommand("sudo /var/vcap/bosh/bin/monit summary | grep 'Process' || true")
		if err != nil {
			return err
		}

		totalProcessesCount := strings.Count(monitProcessList, "Process")
		stoppedProcessesCount := strings.Count(monitProcessList, "not monitored")
		if stoppedProcessesCount == totalProcessesCount {
			monitStopped = true
			break
		}

		time.Sleep(1 * time.Second)
	}

	if !monitStopped {
		return fmt.Errorf("ensureMonitStopped: monit processes not stopped after %.0f seconds", time.Since(begin).Seconds())
	}

	return nil
}

func (t *TestEnvironment) ResetDeviceMap() error {
	out, err := t.RunCommand("sudo losetup -a | cut -f1 -d:")
	if err != nil {
		return err
	}
	for _, loopDev := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		ignoredErr := t.DetachLoopDevice(loopDev)
		if ignoredErr != nil {
			t.writerPrinter.Printf("ResetDeviceMap: %s", ignoredErr)
		}
	}
	_, err = t.RunCommand("sudo rm -f /virtualfs-*")
	if err != nil {
		return err
	}
	t.deviceMap = make(map[int]string)

	return nil
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

func (t *TestEnvironment) EnsureRootDeviceIsLargeEnough() error {
	rootPartition, err := t.RunCommand("sudo findmnt -n -o source -T /")
	if err != nil {
		return err
	}
	rootDevice := rootPartition[:len(rootPartition)-1]

	output, err := t.RunCommand(fmt.Sprintf("sudo parted -m %s unit B print", rootDevice))
	if err != nil {
		return err
	}
	outputLines := strings.Split(strings.Trim(output, "\n"), "\n")[2:]
	rootPartitionFields := strings.Split(outputLines[0], ":")
	sizeInBytes, err := strconv.Atoi(strings.Trim(rootPartitionFields[2], "B"))
	if err != nil {
		return err
	}

	// Ensure we have enough space to create the fake loopback devices used in tests
	if sizeInBytes < 10000000000 {
		_, ignoredErr := t.RunCommand(fmt.Sprintf("sudo swapoff %s", rootDevice))
		if ignoredErr != nil {
			t.writerPrinter.Printf("EnsureRootDeviceIsLargeEnough: %s", ignoredErr)
		}

		for i := len(outputLines); i > 1; i-- {
			_, err = t.RunCommand(fmt.Sprintf("sudo parted %s rm %d", rootDevice, i))
			if err != nil {
				return err
			}
		}

		_, ignoredErr = t.RunCommand("sudo udevadm settle")
		if ignoredErr != nil {
			t.writerPrinter.Printf("EnsureRootDeviceIsLargeEnough: %s", ignoredErr)
		}
		_, err = t.RunCommand("cat /etc/lsb-release | grep -i jammy")
		// parteds behaviour changed and providing yes via params stopped working for jammy.
		// so test if we're running on jammy and adjust parted command
		if err != nil {
			_, err = t.RunCommand(fmt.Sprintf("sudo parted %s ---pretend-input-tty resizepart 1 yes 10000M", rootDevice))
			if err != nil {
				return err
			}
		} else {
			_, err = t.RunCommand(fmt.Sprintf("yes | sudo parted %s ---pretend-input-tty resizepart 1 10000M", rootDevice))
			if err != nil {
				return err
			}
		}
		_, err = t.RunCommand(fmt.Sprintf("sudo resize2fs -f %s", rootDevice))
		if err != nil {
			return err
		}
	}

	return nil
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
	_, err = t.RunCommand(fmt.Sprintf("echo '1,%d,L,' | sudo sfdisk -uS %s", rootPartitionSizeInMB*2048, devicePath))
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
				for _, path := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
					_, ignoredErr := t.RunCommand(fmt.Sprintf("sudo umount -l %s", path))
					if ignoredErr != nil {
						t.writerPrinter.Printf("DetachPartitionedRootDevice: %s", ignoredErr)
					}

				}

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
	_, err := t.RunCommand("sudo losetup -D")
	return err
}

func (t *TestEnvironment) SetUpDummyNetworkInterface(ip, mac string) error {
	return t.RunCommandChain(
		"sudo ip link add dummy0 type dummy",
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
			return err
		}
	}
	return nil
}

func (t *TestEnvironment) TearDownDummyNetworkInterface() error {
	_, err := t.RunCommand("sudo rmmod dummy")
	return err
}

func (t *TestEnvironment) UpdateAgentConfig(configFile string) error {
	_, err := t.RunCommand("sudo rm -f /var/vcap/bosh/agent.json")
	if err != nil {
		return err
	}
	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), configFile), "/var/vcap/bosh/agent.json")
}

func (t *TestEnvironment) CopyFileToPath(localPath string, remotePath string) error {
	_, _, _, err :=
		t.cmdRunner.RunCommand("scp", localPath, fmt.Sprintf("%s:/tmp/remote-file", t.agentIP()))
	if err != nil {
		return err
	}

	_, err = t.RunCommand(fmt.Sprintf("sudo mv /tmp/remote-file %s", remotePath))

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

func (er emptyReader) Read(_ []byte) (int, error) {
	time.Sleep(1 * time.Second)
	return 0, nil
}

func (t *TestEnvironment) StartAgentTunnel() error {
	if t.sshTunnelProc != nil {
		return fmt.Errorf("already running")
	}

	sshCmd := boshsys.Command{
		Name: "ssh",
		Args: []string{
			"-N",
			fmt.Sprintf("-L16868:127.0.0.1:%d", t.mbusPort),
			t.agentIP(),
		},
		Stdin: emptyReader{},
	}
	newTunnelProc, err := t.cmdRunner.RunComplexCommandAsync(sshCmd)
	if err != nil {
		return err
	}
	t.sshTunnelProc = newTunnelProc

	lgr := logger.NewWriterLogger(logger.LevelDebug, t.writerPrinter)

	httpClient := httpclient.NewHTTPClient(httpclient.DefaultClient, lgr)
	t.AgentClient = integrationagentclient.NewIntegrationAgentClient(
		fmt.Sprintf("https://%s:%s@localhost:16868", t.mbusUser, t.mbusPass),
		"fake-director-uuid",
		1*time.Second,
		10,
		httpClient,
		lgr,
	)

	for i := 1; i < 90; i++ {
		t.writerPrinter.Printf("Trying to contact agent via ssh tunnel...")
		time.Sleep(1 * time.Second)
		_, err = t.AgentClient.Ping()
		if err == nil {
			return nil
		}
	}
	t.writerPrinter.Printf("StartAgentTunnel %s", err.Error())
	return err
}

func (t *TestEnvironment) StopAgentTunnel() error {
	if t.sshTunnelProc == nil {
		return nil
	}
	t.sshTunnelProc.Wait()
	ignoredErr := t.sshTunnelProc.TerminateNicely(5 * time.Second)
	if ignoredErr != nil {
		t.writerPrinter.Printf("StopAgentTunnel: %s", ignoredErr)
	}
	t.sshTunnelProc = nil
	return nil
}

func (t *TestEnvironment) StartBlobstore() error {
	_, ignoredErr := t.RunCommand("sudo killall -9 fake-blobstore")
	if ignoredErr != nil {
		t.writerPrinter.Printf("StartBlobstore: %s", ignoredErr)
	}

	_, err :=
		t.RunCommand("nohup /home/agent_test_user/fake-blobstore -host 127.0.0.1 -port 9091 -assets /home/agent_test_user &> /dev/null &")

	return err
}

func (t *TestEnvironment) CreateSettingsFile(settings boshsettings.Settings) error {
	emptyCert := boshsettings.CertKeyPair{}
	if settings.Env.Bosh.Mbus.Cert == emptyCert {
		settings.Env.Bosh.Mbus.Cert.Certificate = agentCert
		settings.Env.Bosh.Mbus.Cert.PrivateKey = agentKey
	}
	if settings.AgentID == "" {
		settings.AgentID = "fake-agent-id"
	}
	if settings.Mbus == "" {
		settings.Mbus = "https://mbus-user:mbus-pass@127.0.0.1:6868"
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(t.AssetsDir(), "test.json"), settingsJSON, 0644)
	if err != nil {
		return err
	}
	_, err = t.RunCommand("sudo rm -f /var/vcap/settings.json")
	if err != nil {
		return err
	}
	_, err = t.RunCommand("sudo rm -f /var/vcap/bosh/settings.json")
	if err != nil {
		return err
	}
	_, err = t.RunCommand("sudo rm -f /var/vcap/bosh/update_settings.json")
	if err != nil {
		return err
	}

	err = t.CopyFileToPath(filepath.Join(t.AssetsDir(), "test.json"), "/var/vcap/settings.json")
	if err != nil {
		return err
	}
	return err
}

func (t *TestEnvironment) GetVMNetworks() (boshsettings.Networks, error) {
	return boshsettings.Networks{
		"eth0": {
			Type: "dynamic",
		},
	}, nil
}

func (t *TestEnvironment) GetFileContents(filePath string) (string, error) {
	return t.RunCommand(fmt.Sprintf("sudo cat %s", filePath))
}

func (t *TestEnvironment) RunCommand(command string) (string, error) {
	s, err := t.sshClient.NewSession()

	if err != nil {
		t.writerPrinter.Println("Remote Cmd Runner", "NewSession() FAILED TO EXECUTE: %s ERROR: %s\n", command, err)
		return "", errors.WrapError(err, "Unable to establish SSH session: ")
	}
	defer s.Close()
	t.writerPrinter.Println("Remote Cmd Runner", "Running remote command '%s'", command)
	out, err := s.CombinedOutput(command)
	if err != nil {
		t.writerPrinter.Println("CombinedOutput() FAILED TO EXECUTE: %s ERROR: %s", command, err)
		return string(out), errors.WrapErrorf(err, "Error running %s", command)
	}
	return string(out), nil
}

func (t *TestEnvironment) CreateSensitiveBlobFromAsset(assetPath, blobID string) error {
	_, err := t.RunCommand("sudo mkdir -p /var/vcap/data/sensitive_blobs")
	if err != nil {
		return err
	}

	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), assetPath), fmt.Sprintf("/var/vcap/data/sensitive_blobs/%s", blobID))
}

func (t *TestEnvironment) CreateBlobFromAsset(assetPath, blobID string) error {
	_, err := t.RunCommand("sudo mkdir -p /var/vcap/data/blobs")
	if err != nil {
		return err
	}

	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), assetPath), fmt.Sprintf("/var/vcap/data/blobs/%s", blobID))
}

func (t *TestEnvironment) CreateBlobFromAssetInActualBlobstore(assetPath, blobstorePath, blobID string) error {
	_, err := t.RunCommand(fmt.Sprintf("sudo mkdir -p %s", blobstorePath))
	if err != nil {
		return err
	}

	return t.CopyFileToPath(filepath.Join(t.AssetsDir(), assetPath), fmt.Sprintf(blobstorePath, blobID))
}

func (t *TestEnvironment) CreateBlobFromStringInActualBlobstore(contents, blobstorePath, blobID string) (string, error) {
	_, err := t.RunCommand(fmt.Sprintf("sudo mkdir -p %s", blobstorePath))
	if err != nil {
		return "", err
	}

	remoteBlobPath := filepath.Join(blobstorePath, blobID)
	_, _, _, err = t.cmdRunner.RunCommandWithInput(
		contents,
		"ssh",
		t.agentIP(),
		fmt.Sprintf("cat | sudo tee %s", remoteBlobPath),
	)
	if err != nil {
		return "", err
	}

	blobDigest, _, _, err := t.cmdRunner.RunCommand(
		"ssh",
		t.agentIP(),
		fmt.Sprintf("sudo shasum %s | cut -f 1 -d ' '", remoteBlobPath),
	)

	return blobDigest, err
}

func (t *TestEnvironment) agentDir() string {
	integrationPath, _ := os.Getwd()
	agentDir, _ := filepath.Split(integrationPath)
	return agentDir
}

func (t *TestEnvironment) agentIP() string {
	return os.Getenv("AGENT_IP")
}

func (t *TestEnvironment) AssetsDir() string {
	return filepath.Join(t.agentDir(), "integration", "assets")
}

func (t *TestEnvironment) BlobstoreDir() string {
	return "/home/agent_test_user"
}

func dialSSHClient(cmdRunner boshsys.CmdRunner) (*ssh.Client, error) {
	stdout, _, _, err := cmdRunner.RunCommand("cat", "ssh-config")
	if err != nil {
		return nil, err
	}
	config, err := ssh_config.Decode(strings.NewReader(stdout))
	if err != nil {
		return nil, err
	}
	user, err := config.Get("agent_vm", "User")
	if err != nil {
		return nil, err
	}
	addr, err := config.Get("agent_vm", "HostName")
	if err != nil {
		return nil, err
	}
	port, err := config.Get("agent_vm", "Port")
	if err != nil {
		return nil, err
	}
	keyPath, err := config.Get("agent_vm", "IdentityFile")
	if err != nil {
		return nil, err
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	testVMAddress := fmt.Sprintf("%s:%s", addr, port)
	testVMSSHConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	jumpboxAddr, err := config.Get("jumpbox", "HostName")
	if err != nil {
		return nil, err
	}
	if jumpboxAddr != "" {
		jumpboxUser, err := config.Get("jumpbox", "User")
		if err != nil {
			return nil, err
		}
		jumpboxKeyPath, err := config.Get("jumpbox", "IdentityFile")
		if err != nil {
			return nil, err
		}
		jumpboxKey, err := os.ReadFile(jumpboxKeyPath)
		if err != nil {
			return nil, err
		}
		jumpboxSigner, err := ssh.ParsePrivateKey(jumpboxKey)
		if err != nil {
			return nil, err
		}

		jumpboxClient, err :=
			ssh.Dial("tcp", fmt.Sprintf("%s:%s", jumpboxAddr, "22"), &ssh.ClientConfig{
				User: jumpboxUser,
				Auth: []ssh.AuthMethod{
					ssh.PublicKeys(jumpboxSigner),
				},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			})
		if err != nil {
			return nil, err
		}

		proxyConnection, err := jumpboxClient.Dial("tcp", testVMAddress)
		if err != nil {
			return nil, err
		}

		proxyClientConnection, proxyClientChannel, proxyClientRequest, err :=
			ssh.NewClientConn(proxyConnection, testVMAddress, testVMSSHConfig)
		if err != nil {
			return nil, err
		}

		return ssh.NewClient(proxyClientConnection, proxyClientChannel, proxyClientRequest), nil
	}
	return ssh.Dial("tcp", testVMAddress, testVMSSHConfig)
}
