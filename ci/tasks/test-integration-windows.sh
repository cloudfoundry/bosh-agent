#!/usr/bin/env bash

set -eu -o pipefail

CONCOURSE_ROOT=$(pwd)

bosh_agent_dir="${CONCOURSE_ROOT}/bosh-agent"
agent_vm_key_path="${CONCOURSE_ROOT}/agent-info/agent-creds.pem"
agent_ip_path="${CONCOURSE_ROOT}/agent-info/agent_ip"
fake_director_ip_path="${CONCOURSE_ROOT}/agent-info/fake_director_ip"
jumpbox_key_path="${CONCOURSE_ROOT}/jumpbox-key.pem"
nats_ca_path="${CONCOURSE_ROOT}/agent-info/nats-ca.pem"
nats_certificate_path="${CONCOURSE_ROOT}/agent-info/nats-certificate.pem"
nats_private_key_path="${CONCOURSE_ROOT}/agent-info/nats-private-key.pem"

mkdir -p ~/.ssh

echo "${JUMPBOX_PRIVATE_KEY}" > "${jumpbox_key_path}"
chmod 600 "${jumpbox_key_path}"

agent_ip="$(cat "${agent_ip_path}")"
fake_director_ip="$(cat "${fake_director_ip_path}")"

echo "
Host ${JUMPBOX_IP}
  User ${JUMPBOX_USERNAME}
  IdentityFile ${jumpbox_key_path}

Host ${agent_ip}
  User vcap
  IdentityFile ${agent_vm_key_path}
  ProxyJump ${JUMPBOX_IP}
" > ~/.ssh/config

ssh-keyscan -H "${JUMPBOX_IP}" >> ~/.ssh/known_hosts 2>/dev/null
ssh "${JUMPBOX_USERNAME}@${JUMPBOX_IP}" "ssh-keyscan -H ${agent_ip}" >> ~/.ssh/known_hosts 2>/dev/null

echo -e "\n Enabling WinRM and setting vcap password..."
ssh "${agent_ip}" "powershell.exe -noprofile -command Enable-WinRM" > /dev/null 2>&1
ssh "${agent_ip}" "NET.exe USER vcap Agent-test-password1" > /dev/null 2>&1

echo -e "\n Stopping running agent processes..."
ssh "${agent_ip}" "c:\bosh\service_wrapper.exe stop" > /dev/null 2>&1
ssh "${agent_ip}" "c:\bosh\service_wrapper.exe uninstall" > /dev/null 2>&1
ssh "${agent_ip}" "powershell.exe -noprofile -command Stop-Service -Name bosh-dns-healthcheck-windows" > /dev/null 2>&1
ssh "${agent_ip}" "powershell.exe -noprofile -command Stop-Service -Name bosh-dns-nameserverconfig-windows" > /dev/null 2>&1
ssh "${agent_ip}" "powershell.exe -noprofile -command Stop-Service -Name bosh-dns-windows" > /dev/null 2>&1

pushd "${bosh_agent_dir}" > /dev/null
  pushd main
    echo -e "\n Building bosh-agent.exe ..."
    GOOS=windows go build -o "${bosh_agent_dir}/integration/windows/fixtures/bosh-agent.exe"
  popd
  pushd jobsupervisor/pipe
    echo -e "\n Building pipe.exe ..."
    GOOS=windows go build -o "${bosh_agent_dir}/integration/windows/fixtures/pipe.exe"
  popd

  echo -e "\n Installing agent and fixtures..."
  set -x
  scp -r "${bosh_agent_dir}"/integration/windows/fixtures/* "${agent_ip}":/bosh/
  ssh "${agent_ip}" 'move /Y C:\bosh\pipe.exe C:\var\vcap\bosh\bin\pipe.exe'
  ssh "${agent_ip}" 'move /Y C:\bosh\psFixture "C:\Program Files\WindowsPowerShell\Modules\"'
  set +x

  echo -e "\n Running tests..."
  export AGENT_IP=${agent_ip}
  export FAKE_DIRECTOR_IP=${fake_director_ip}
  export JUMPBOX_IP=${JUMPBOX_IP}
  export JUMPBOX_USERNAME=${JUMPBOX_USERNAME}
  export JUMPBOX_KEY_PATH=${jumpbox_key_path}
  export NATS_CA_PATH=${nats_ca_path}
  export NATS_CERTIFICATE_PATH=${nats_certificate_path}
  export NATS_PRIVATE_KEY_PATH=${nats_private_key_path}

  go run github.com/onsi/ginkgo/v2/ginkgo run -vv integration/windows/
popd
