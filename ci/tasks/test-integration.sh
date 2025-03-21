#!/usr/bin/env bash

set -eu -o pipefail

# To run locally set the following:
#   LOCAL_AGENT_IP
#   LOCAL_AGENT_VM_KEY_PATH
#   LOCAL_JUMPBOX_KEY_PATH
# ... probably works ¯\_(ツ)_/¯

function copy_to_remote_host() {
  local_file=${1}
  remote_path=${2}
  scp_flag=${3:-}

  scp ${scp_flag} "${local_file}" "${agent_ip}:/tmp/remote-file" > /dev/null 2>&1
  ${ssh_command} "sudo mv /tmp/remote-file ${remote_path}" > /dev/null 2>&1
  ${ssh_command} "sudo rm -rf /tmp/remote-file" > /dev/null 2>&1
}

CONCOURSE_ROOT=$(pwd)
bosh_agent_dir="${CONCOURSE_ROOT}/bosh-agent"
agent_ip_path="${CONCOURSE_ROOT}/agent-info/agent_ip"

agent_ip="${LOCAL_AGENT_IP:-$(cat "${agent_ip_path}")}"
agent_vm_key_path=${LOCAL_AGENT_VM_KEY_PATH:-"${CONCOURSE_ROOT}/agent-info/agent-creds.pem"}
jumpbox_key_path=${LOCAL_JUMPBOX_KEY_PATH:-"${CONCOURSE_ROOT}/jumpbox-key.pem"}

mkdir -p ~/.ssh

echo "${JUMPBOX_PRIVATE_KEY}" > "${jumpbox_key_path}"
chmod 600 "${jumpbox_key_path}"

echo -e "\n Creating agent_test_user"
export BOSH_ALL_PROXY="ssh+socks5://${JUMPBOX_USERNAME}@${JUMPBOX_IP}:22?private-key=${jumpbox_key_path}"
bosh ssh -c "sudo useradd agent_test_user &&
sudo usermod -G bosh_sshers,bosh_sudoers agent_test_user &&
sudo usermod -s /bin/bash agent_test_user &&
sudo mkdir -p /home/agent_test_user/.ssh &&
sudo cp /home/vcap/.ssh/authorized_keys /home/agent_test_user/.ssh/authorized_keys &&
sudo chown -R agent_test_user:agent_test_user /home/agent_test_user/"
unset BOSH_ALL_PROXY


echo "
Host ${JUMPBOX_IP}
  User ${JUMPBOX_USERNAME}
  IdentityFile ${jumpbox_key_path}
Host ${agent_ip}
  User agent_test_user
  IdentityFile ${agent_vm_key_path}
  ProxyJump ${JUMPBOX_IP}
" > ~/.ssh/config

ssh-keyscan -H "${JUMPBOX_IP}" >> ~/.ssh/known_hosts 2>/dev/null
# shellcheck disable=SC2029
ssh "${JUMPBOX_USERNAME}@${JUMPBOX_IP}" "ssh-keyscan -H ${agent_ip}" >> ~/.ssh/known_hosts 2>/dev/null
ssh_command="ssh ${agent_ip}"

pushd "${bosh_agent_dir}"
  echo -e "\n Building agent..."
  GOARCH=amd64 GOOS=linux bin/build
popd

echo -e "\n Installing agent..."
${ssh_command} "sudo sv stop agent" >/dev/null 2>&1
copy_to_remote_host "${bosh_agent_dir}/out/bosh-agent" /var/vcap/bosh/bin/bosh-agent

echo -e "\n Shutting down rsyslog..."
${ssh_command}  "sudo systemctl disable --now syslog.socket rsyslog.service" >/dev/null 2>&1

echo -e "\n Installing fake blobstore..."
pushd "${bosh_agent_dir}/integration/fake-blobstore"
  GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build .
  copy_to_remote_host ./fake-blobstore /home/agent_test_user/fake-blobstore
popd


echo -e "\n Setup assets"
pushd "${bosh_agent_dir}/integration/assets"
  release_folder="/home/agent_test_user/release"
  ${ssh_command} "sudo rm -rf ${release_folder}" > /dev/null 2>&1
  copy_to_remote_host release /home/agent_test_user/release -r
popd

echo -e "\n Running agent integration tests..."
pushd "${bosh_agent_dir}"
  echo "
Host agent_vm
User agent_test_user
Hostname ${agent_ip}
Port 22
IdentityFile ${agent_vm_key_path}

Host jumpbox
User ${JUMPBOX_USERNAME}
Hostname ${JUMPBOX_IP}
Port 22
IdentityFile ${jumpbox_key_path}
" > integration/ssh-config

  export AGENT_IP="${agent_ip}"
  go run github.com/onsi/ginkgo/v2/ginkgo --trace integration
popd


