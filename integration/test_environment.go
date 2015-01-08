package integration

import (
	"encoding/json"
	"fmt"
	"strconv"

	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type RegistrySettings struct {
	AgentID string `json:"agent_id"`
}

type TestEnvironment struct {
	cmdRunner boshsys.CmdRunner
}

func NewTestEnvironment(
	cmdRunner boshsys.CmdRunner,
) TestEnvironment {
	return TestEnvironment{
		cmdRunner: cmdRunner,
	}
}

func (t TestEnvironment) SetInfrastructure(name string) error {
	_, err := t.RunCommand(fmt.Sprintf("echo '%s' | sudo tee /var/vcap/bosh/etc/infrastructure", name))
	return err
}

func (t TestEnvironment) SetupConfigDrive() error {
	setupConfigDriveTemplate := `
export GOPATH=/home/vagrant/go
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$PATH

sudo dd if=/dev/zero of=/virtualfs bs=1024 count=1024
sudo losetup /dev/loop2 /virtualfs
sudo mkfs -t ext3 -m 1 -v /dev/loop2
sudo e2label /dev/loop2 config-2
sudo rm -rf /tmp/config-drive
sudo mkdir /tmp/config-drive
sudo mount /dev/disk/by-label/config-2 /tmp/config-drive
sudo chown vagrant:vagrant /tmp/config-drive
sudo mkdir -p /tmp/config-drive/ec2/latest
sudo cp %s/meta-data.json /tmp/config-drive/ec2/latest/meta-data.json
sudo cp %s/user-data /tmp/config-drive/ec2/latest
sudo umount /tmp/config-drive
`
	setupConfigDriveScript := fmt.Sprintf(setupConfigDriveTemplate, t.assetsDir(), t.assetsDir())

	_, err := t.RunCommand(setupConfigDriveScript)
	return err
}

func (t TestEnvironment) RemoveAgentSettings() error {
	_, err := t.RunCommand("sudo rm -f /var/vcap/bosh/settings.json")
	return err
}

func (t TestEnvironment) UpdateAgentConfig(configFile string) error {
	_, err := t.RunCommand(
		fmt.Sprintf(
			"sudo cp %s/%s /var/vcap/bosh/agent.json",
			t.assetsDir(),
			configFile,
		),
	)
	return err
}

func (t TestEnvironment) RestartAgent() error {
	_, err := t.RunCommand("nohup sudo sv stop agent &")
	if err != nil {
		return err
	}

	_, err = t.RunCommand("nohup sudo sv start agent &")
	return err
}

func (t TestEnvironment) StartRegistry(settings RegistrySettings) error {
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	_, err = t.RunCommand(
		fmt.Sprintf(
			`nohup %s/tmp/fake-registry -user user -password pass -host localhost -port 9090 -instance instance-id -settings %s &> /dev/null &`,
			t.agentDir(),
			strconv.Quote(string(settingsJSON)),
		),
	)
	return err
}

func (t TestEnvironment) GetFileContents(filePath string) (string, error) {
	return t.RunCommand(
		fmt.Sprintf(
			`cat %s`,
			filePath,
		),
	)
}

func (t TestEnvironment) RunCommand(command string) (string, error) {
	stdout, _, _, err := t.cmdRunner.RunCommand(
		"vagrant",
		"ssh",
		"-c",
		command,
	)

	return stdout, err
}

func (t TestEnvironment) agentDir() string {
	return "/home/vagrant/go/src/github.com/cloudfoundry/bosh-agent"
}

func (t TestEnvironment) assetsDir() string {
	return fmt.Sprintf("%s/integration/assets", t.agentDir())
}
