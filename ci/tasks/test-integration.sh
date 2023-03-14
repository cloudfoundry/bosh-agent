#!/usr/bin/env bash

set -eu -o pipefail

function copy_to_remote_host() {
  local_file=$1
  remote_path=$2
  def_arg=""

  scp ${3:-$def_arg} ${local_file} ${agent_ip}:/tmp/remote-file > /dev/null 2>&1
  ${ssh_command} "sudo mv /tmp/remote-file ${remote_path}" > /dev/null 2>&1
  ${ssh_command} "sudo rm -rf /tmp/remote-file" > /dev/null 2>&1
}

script_dir=$(dirname "$0")
bosh_agent_dir=$( cd "${script_dir}"/../.. && pwd )
workspace_dir=$( cd "${bosh_agent_dir}"/.. && pwd )
agent_vm_key_path="${workspace_dir}/agent-info/agent-creds.pem"
agent_ip_path="${workspace_dir}/agent-info/agent_ip"
jumpbox_key_path="${workspace_dir}/jumpbox-key.pem"

mkdir -p ~/.ssh

echo "${JUMPBOX_PRIVATE_KEY}" > ${jumpbox_key_path}
chmod 600 ${jumpbox_key_path}

deployment_name="bosh-agent-integration"

agent_ip="$(cat ${agent_ip_path})"

export BOSH_ALL_PROXY="ssh+socks5://${JUMPBOX_USERNAME}@${JUMPBOX_IP}:22?private-key=${jumpbox_key_path}"

echo -e "\n Creating agent_test_user"
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

ssh-keyscan -H ${JUMPBOX_IP} >> ~/.ssh/known_hosts 2>/dev/null
ssh ${JUMPBOX_USERNAME}@${JUMPBOX_IP} "ssh-keyscan -H ${agent_ip}" >> ~/.ssh/known_hosts 2>/dev/null
ssh_command="ssh ${agent_ip}"

cd ${bosh_agent_dir}
echo -e "\n Building agent..."
GOOS=linux bin/build

echo -e "\n Installing agent..."
${ssh_command} "sudo sv stop agent" >/dev/null 2>&1
copy_to_remote_host ${bosh_agent_dir}/out/bosh-agent /var/vcap/bosh/bin/bosh-agent


echo -e "\n Shutting down rsyslog..."
${ssh_command}  "sudo systemctl disable --now syslog.socket rsyslog.service" >/dev/null 2>&1

echo -e "\n Installing fake blobstore..."
pushd ${bosh_agent_dir}/integration/fake-blobstore
  export CGO_ENABLED=0
  go build .
  copy_to_remote_host ./fake-blobstore /home/agent_test_user/fake-blobstore
popd


echo -e "\n Setup assets"
pushd ${bosh_agent_dir}/integration/assets
  release_folder="/home/agent_test_user/release"
  ${ssh_command} "sudo rm -rf ${release_folder}" > /dev/null 2>&1
  copy_to_remote_host release /home/agent_test_user/release -r
popd

echo -e "\n Running agent integration tests..."
pushd ${bosh_agent_dir}
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

  export AGENT_IP=${agent_ip}
  go run github.com/onsi/ginkgo/ginkgo --slowSpecThreshold=300 --trace --progress integration
popd


