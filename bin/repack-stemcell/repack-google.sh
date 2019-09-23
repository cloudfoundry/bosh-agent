#!/bin/bash -eu

if [ $# != 2 ]; then
  echo "USAGE: repack-google-stemcell [base-stemcell] [updated-version-number]"
  exit 2
fi

stemcell_path=$1
VERSION=$2

input_stemcell=$(mktemp -d)
input_disk=$(mktemp -d)
chroot=$(mktemp -d)
output_stemcell=$(mktemp -d)

cleanup() {
  rm -rf $input_stemcell
  rm -rf $input_disk
  rm -rf $chroot
}

trap cleanup EXIT

echo "Building agent..."
bosh_agent=$(dirname $0)/../..
$bosh_agent/bin/build-linux-amd64

echo "Extracting input stemcell..."
tar -xzf ${stemcell_path} -C $input_stemcell
tar -xzf ${input_stemcell}/image -C ${input_disk}
sudo losetup -fP ${input_disk}/disk.raw
loopback=$(losetup -a | grep disk.raw | cut -d ':' -f1)
sudo mount -o loop,rw ${loopback}p1 ${chroot}

echo "Copying over bosh agent..."
sudo cp $bosh_agent/out/bosh-agent ${chroot}/var/vcap/bosh/bin/

echo "Repacking stemcell..."
sudo umount ${chroot}
sudo losetup -d ${loopback}
pushd ${input_disk}
  tar czf ${input_stemcell}/image disk.raw
popd
pushd ${input_stemcell}
  sed -i -e "/version:/d" stemcell.MF
  echo "version: $VERSION" >> stemcell.MF
  tar czf ${output_stemcell}/stemcell.tgz *
popd

echo "DONE..."
echo "Your stemcell can be found at: ${output_stemcell}/stemcell.tgz"


echo "Next manual steps for unresponsive agents:

* Upload to your environment
* Run a deploy
* 'Create Image' from GCP console off the broken VM
  * gcloud compute images create ja-as-aug-12-image --source-disk="vm-9f74f49a-bb3a-4df8-4e74-f26baecc175f" --source-disk-zone us-west1-a --force
* 'Create Disk' from this image (IMPORTANT: IN THE SAME ZONE)
  * gcloud compute disks create ja-as-aug-12-disk --image-project cf-bosh-core --image ja-as-aug-12-image --zone us-west1-a
* Attach on a healthy VM
  * gcloud compute instances attach-disk vm-9f74f49a-bb3a-4df8-4e74-f26baecc175f --disk="ja-as-aug-12-disk"
* SSH on healthy VM
* run 'mkdir -p /rootdisk2 && mount <drive> /rootdisk2'
* have fun"
