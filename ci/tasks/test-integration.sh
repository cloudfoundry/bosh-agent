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

echo "
Host ${JUMPBOX_IP}
  User ${JUMPBOX_USERNAME}
  IdentityFile ${jumpbox_key_path}

Host ${agent_ip}
  User vcap
  IdentityFile ${agent_vm_key_path}
  ProxyJump ${JUMPBOX_IP}
" > ~/.ssh/config

ssh_command="ssh ${agent_ip}"

ssh-keyscan -H ${JUMPBOX_IP} >> ~/.ssh/known_hosts 2>/dev/null
ssh ${JUMPBOX_USERNAME}@${JUMPBOX_IP} "ssh-keyscan -H ${agent_ip}" >> ~/.ssh/known_hosts 2>/dev/null

echo -e "\n Creating agent_test_user"
${ssh_command} "sudo useradd agent_test_user" >/dev/null 2>&1
${ssh_command} "sudo usermod -G bosh_sshers,bosh_sudoers agent_test_user" >/dev/null 2>&1
${ssh_command} "sudo usermod -s /bin/bash agent_test_user" >/dev/null 2>&1
${ssh_command} "sudo mkdir -p /home/agent_test_user/.ssh" >/dev/null 2>&1
${ssh_command} "sudo cp /home/vcap/.ssh/authorized_keys /home/agent_test_user/.ssh/authorized_keys" >/dev/null 2>&1
${ssh_command} "sudo chown -R agent_test_user:agent_test_user /home/agent_test_user/" >/dev/null 2>&1

echo "
Host ${JUMPBOX_IP}
  User ${JUMPBOX_USERNAME}
  IdentityFile ${jumpbox_key_path}
Host ${agent_ip}
  User agent_test_user
  IdentityFile ${agent_vm_key_path}
  ProxyJump ${JUMPBOX_IP}
" > ~/.ssh/config

cd ${bosh_agent_dir}
echo -e "\n Building agent..."
bin/build

echo -e "\n Installing agent..."
${ssh_command} "sudo sv stop agent" >/dev/null 2>&1
copy_to_remote_host ${bosh_agent_dir}/out/bosh-agent /var/vcap/bosh/bin/bosh-agent

echo -e "\n Installing fake registry..."
pushd ${bosh_agent_dir}/integration/fake-registry
  go build .
  copy_to_remote_host ./fake-registry /home/agent_test_user/fake-registry
popd

echo -e "\n Installing fake blobstore..."
pushd ${bosh_agent_dir}/integration/fake-blobstore
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


