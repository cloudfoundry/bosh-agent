#!/usr/bin/env bash

set -eu -o pipefail

script_dir=$(dirname "$0")
bosh_agent_dir=$( cd "${script_dir}"/../.. && pwd )
workspace_dir=$( cd "${bosh_agent_dir}"/.. && pwd )

agent_vm_key_path="${workspace_dir}/agent-info/agent-creds.pem"
agent_ip_path="${workspace_dir}/agent-info/agent_ip"
fake_director_ip_path="${workspace_dir}/agent-info/fake_director_ip"
jumpbox_key_path="${workspace_dir}/jumpbox-key.pem"
nats_ca_path="${workspace_dir}/agent-info/nats-ca.pem"
nats_certificate_path="${workspace_dir}/agent-info/nats-certificate.pem"
nats_private_key_path="${workspace_dir}/agent-info/nats-private-key.pem"

mkdir -p ~/.ssh

echo "${JUMPBOX_PRIVATE_KEY}" > ${jumpbox_key_path}
chmod 600 ${jumpbox_key_path}

agent_ip="$(cat ${agent_ip_path})"
fake_director_ip="$(cat ${fake_director_ip_path})"

echo "
Host ${JUMPBOX_IP}
  User ${JUMPBOX_USERNAME}
  IdentityFile ${jumpbox_key_path}

Host ${agent_ip}
  User vcap
  IdentityFile ${agent_vm_key_path}
  ProxyJump ${JUMPBOX_IP}
" > ~/.ssh/config

ssh-keyscan -H ${JUMPBOX_IP} >> ~/.ssh/known_hosts 2>/dev/null
ssh ${JUMPBOX_USERNAME}@${JUMPBOX_IP} "ssh-keyscan -H ${agent_ip}" >> ~/.ssh/known_hosts 2>/dev/null

echo -e "\n Enabling WinRM and setting vcap password..."
ssh ${agent_ip} "powershell.exe -noprofile -command Enable-WinRM" > /dev/null 2>&1
ssh ${agent_ip} "NET.exe USER vcap Agent-test-password1" > /dev/null 2>&1

echo -e "\n Stopping running agent processes..."
ssh ${agent_ip} "c:\bosh\service_wrapper.exe stop" > /dev/null 2>&1
ssh ${agent_ip} "c:\bosh\service_wrapper.exe uninstall" > /dev/null 2>&1
ssh ${agent_ip} "powershell.exe -noprofile -command Stop-Service -Name bosh-dns-healthcheck-windows" > /dev/null 2>&1
ssh ${agent_ip} "powershell.exe -noprofile -command Stop-Service -Name bosh-dns-nameserverconfig-windows" > /dev/null 2>&1
ssh ${agent_ip} "powershell.exe -noprofile -command Stop-Service -Name bosh-dns-windows" > /dev/null 2>&1

pushd ${bosh_agent_dir} > /dev/null
  echo -e "\n Building agent..."
  agent_output_path="${bosh_agent_dir}/integration/windows/fixtures/bosh-agent.exe"
  pipe_output_path="${bosh_agent_dir}/integration/windows/fixtures/pipe.exe"

  pushd main > /dev/null
    GOOS=windows go build -o "${agent_output_path}"
  popd > /dev/null
  pushd jobsupervisor/pipe > /dev/null
    GOOS=windows go build -o "${pipe_output_path}"
  popd > /dev/null

  echo -e "\n Installing agent and fixtures..."
  scp -r ${bosh_agent_dir}/integration/windows/fixtures/* ${agent_ip}:/bosh/ > /dev/null 2>&1
  ssh ${agent_ip} 'move /Y C:\bosh\pipe.exe C:\var\vcap\bosh\bin\pipe.exe' > /dev/null 2>&1
  ssh ${agent_ip} 'move /Y C:\bosh\psFixture "C:\Program Files\WindowsPowerShell\Modules\"' > /dev/null 2>&1

  echo -e "\n Running tests..."
  export AGENT_IP=${agent_ip}
  export FAKE_DIRECTOR_IP=${fake_director_ip}
  export JUMPBOX_IP=${JUMPBOX_IP}
  export JUMPBOX_USERNAME=${JUMPBOX_USERNAME}
  export JUMPBOX_KEY_PATH=${jumpbox_key_path}
  export NATS_CA_PATH=${nats_ca_path}
  export NATS_CERTIFICATE_PATH=${nats_certificate_path}
  export NATS_PRIVATE_KEY_PATH=${nats_private_key_path}
  go run github.com/onsi/ginkgo/ginkgo -race --slowSpecThreshold=300 -trace integration/windows/
popd > /dev/null
