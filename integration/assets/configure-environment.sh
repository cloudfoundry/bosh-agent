#!/bin/bash

# Assume to be running on bosh lite
set -x -e

export GOPATH=/home/vagrant/go
export GOROOT=/usr/local/go
export PATH=$GOROOT/bin:$PATH

agent_dir=$GOPATH/src/github.com/cloudfoundry/bosh-agent
assets_dir=$agent_dir/integration/assets

# stop the currently running agent
sudo sv stop agent

# remove settings.json
sudo mv /var/vcap/bosh/settings.json{,.bk}

# prepare config-drive
sudo dd if=/dev/zero of=/virtualfs bs=1024 count=1024
sudo losetup /dev/loop2 /virtualfs
sudo mkfs -t ext3 -m 1 -v /dev/loop2
sudo e2label /dev/loop2 config-2
sudo mkdir /tmp/config-drive
sudo mount /dev/disk/by-label/config-2 /tmp/config-drive
sudo chown vagrant:vagrant /tmp/config-drive
sudo mkdir -p /tmp/config-drive/ec2/latest
sudo cp $assets_dir/meta-data.json /tmp/config-drive/ec2/latest/meta-data.json
sudo cp $assets_dir/user-data /tmp/config-drive/ec2/latest
sudo umount /tmp/config-drive

# change the agent's infrastructure
sudo mv /var/vcap/bosh/etc/infrastructure{,.bk}
echo 'openstack' | sudo tee /var/vcap/bosh/etc/infrastructure

# get agent settings from test assets
sudo cp $assets_dir/agent.json /var/vcap/bosh/agent.json

pushd $agent_dir
	# install golang
	sudo $agent_dir/integration/assets/install-go.sh

	# build fake registry
	./integration/bin/build

	# start registry and seed it with a new agent_id
	nohup tmp/fake-registry -user user -password pass -host localhost -port 9090 -instance instance-id -settings "{\"agent_id\":\"the_agent_id\"}" &> /dev/null &

	# build agent
	bin/build

	# install new agent
	sudo cp out/bosh-agent /var/vcap/bosh/bin/bosh-agent

	# start agent
	sudo sv start agent
popd
