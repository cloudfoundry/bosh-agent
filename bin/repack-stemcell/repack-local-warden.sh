#!/bin/bash

set -e -x

env
bosh_agent_bin=$(dirname $0)
cd $bosh_agent_bin/../..
./bin/build-linux-amd64

bosh_agent_src=$(pwd)

stemcell_tgz=/tmp/stemcell.tgz
stemcell_dir=/tmp/stemcell
image_dir=/tmp/image

sudo rm -rf $stemcell_dir
sudo rm -rf $image_dir
mkdir -p $stemcell_dir $image_dir
wget -O- $STEMCELL_URL > $stemcell_tgz
echo "$STEMCELL_SHA1  $stemcell_tgz" | shasum -c -

# Repack stemcell
(
	set -e;
	cd $stemcell_dir
	sudo tar xvf $stemcell_tgz
	new_ver=`date +%s`

	# Update stemcell with new agent
	(
		set -e;
		cd $image_dir
		sudo tar xvf $stemcell_dir/image
    sudo bash -c "echo -n 0.0.${new_ver} > $image_dir/var/vcap/bosh/etc/stemcell_version"
		sudo cp $bosh_agent_src/out/bosh-agent $image_dir/var/vcap/bosh/bin/bosh-agent

		sudo tar czvf $stemcell_dir/image *
	)

	sudo sed -i.bak "s/version: .*/version: 0.0.${new_ver}/" stemcell.MF
	sudo tar czvf $stemcell_tgz *
)

echo "Find your new stemcell at $stemcell_tgz"
