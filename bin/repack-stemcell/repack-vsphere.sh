#!/bin/bash -eu
#
# Repack a vSphere stemcell with a locally compiled bosh-agent binary.
#
# This script must run on Linux because it mounts disk images. On macOS, run it
# inside the bosh/agent Docker image, which already has the right Go toolchain.
# The bosh-agent source tree is bind-mounted into the container so the freshly
# compiled binary is used.
#
# --- macOS usage ---
#
# 1. Download a full (non-light) vSphere stemcell, e.g.:
#      curl -L -o /tmp/bosh-stemcell.tgz \
#        https://bosh.io/d/stemcells/bosh-vsphere-esxi-ubuntu-jammy-go_agent
#
# 2. Run from the root of the bosh-agent checkout:
#      docker run --privileged \
#        -v $(pwd):/bosh-agent \
#        -v /tmp:/stemcell \
#        -w /bosh-agent \
#        bosh/agent \
#        bash -c "apt-get install -y -q qemu-utils 2>&1 | tail -3 && \
#                 bin/repack-stemcell/repack-vsphere.sh \
#                   /stemcell/bosh-stemcell.tgz <new-version> && \
#                 cp /tmp/stemcell-vsphere-repacked.tgz /stemcell/"
#
#    Replace <new-version> with any version string, e.g. 1.1202.1
#    --privileged is required for loop device mounts.
#
# The repacked stemcell is written to /tmp/stemcell-vsphere-repacked.tgz
# (inside the container). The example above copies it back to /tmp on the host.

if [ $# != 2 ]; then
  echo "USAGE: repack-vsphere.sh <base-stemcell.tgz> <new-version>"
  exit 2
fi

stemcell_path=$(realpath "$1")
VERSION="$2"

input_stemcell=$(mktemp -d)
input_disk=$(mktemp -d)
chroot_dir=$(mktemp -d)
output_stemcell=$(mktemp -d)
raw_disk=$(mktemp)
output_path=/tmp/stemcell-vsphere-repacked.tgz

cleanup() {
  if mountpoint -q "$chroot_dir" 2>/dev/null; then
    umount "$chroot_dir"
  fi
  rm -f "$raw_disk"
  rm -rf "$input_stemcell" "$input_disk" "$chroot_dir" "$output_stemcell"
}
trap cleanup EXIT

echo "Building agent..."
bosh_agent_root="$(dirname "$0")/../.."
"$bosh_agent_root/bin/build-linux-amd64"
agent_bin="$bosh_agent_root/out/bosh-agent"

echo "Extracting input stemcell..."
tar -xzf "$stemcell_path" -C "$input_stemcell"
tar -xzf "$input_stemcell/image" -C "$input_disk"

vmdk=$(find "$input_disk" -name '*.vmdk' | head -1)
if [ -z "$vmdk" ]; then
  echo "ERROR: no .vmdk found inside stemcell image"
  exit 1
fi
echo "Found disk image: $vmdk"

# vSphere stemcells use stream-optimized VMDKs which can't be directly
# block-device-mounted. Convert to raw, read the partition offset with python3
# (guaranteed present as a qemu-utils dependency), then mount with an explicit
# offset — no partition device nodes needed.
echo "Converting VMDK to raw (this may take a minute)..."
qemu-img convert -f vmdk -O raw "$vmdk" "$raw_disk"

echo "Finding partition offset..."
# Finds the Linux root filesystem partition (not the EFI boot partition).
# Handles both GPT (looks for Linux filesystem type GUID) and MBR (type 0x83).
start_sector=$(python3 - "$raw_disk" <<'PYEOF'
import struct, sys

# Linux filesystem type GUID (mixed-endian as stored on disk):
# 0FC63DAF-8483-4772-8E79-3D69D8477DE4
LINUX_FS_GUID = bytes([0xAF,0x3D,0xC6,0x0F, 0x83,0x84, 0x72,0x47,
                        0x8E,0x79, 0x3D,0x69,0xD8,0x47,0x7D,0xE4])

with open(sys.argv[1], 'rb') as f:
    header = f.read(1024)

if header[512:520] == b'EFI PART':
    part_entry_lba = struct.unpack_from('<Q', header, 512 + 72)[0]
    num_entries    = struct.unpack_from('<I', header, 512 + 80)[0]
    entry_size     = struct.unpack_from('<I', header, 512 + 84)[0]
    with open(sys.argv[1], 'rb') as f:
        f.seek(part_entry_lba * 512)
        table = f.read(num_entries * entry_size)
    # Prefer the Linux filesystem partition; fall back to first non-empty entry
    fallback = None
    for i in range(num_entries):
        e = table[i*entry_size:(i+1)*entry_size]
        start = struct.unpack_from('<Q', e, 32)[0]
        if start == 0:
            continue
        if fallback is None:
            fallback = start
        if e[0:16] == LINUX_FS_GUID:
            print(start)
            sys.exit(0)
    print(fallback)
else:
    # MBR: prefer type 0x83 (Linux), fall back to first non-empty entry
    fallback = None
    for i in range(4):
        e = header[446 + i*16 : 446 + (i+1)*16]
        ptype = e[4]
        start = struct.unpack_from('<I', e, 8)[0]
        if start == 0:
            continue
        if fallback is None:
            fallback = start
        if ptype == 0x83:
            print(start)
            sys.exit(0)
    print(fallback)
PYEOF
)
if [ -z "$start_sector" ] || [ "$start_sector" -eq 0 ]; then
  echo "ERROR: could not determine partition offset"
  exit 1
fi
offset=$((start_sector * 512))
echo "Partition start: sector $start_sector (offset $offset bytes)"

mount -o loop,offset="$offset" "$raw_disk" "$chroot_dir"

echo "Copying new bosh-agent..."
cp "$agent_bin" "$chroot_dir/var/vcap/bosh/bin/bosh-agent"
chmod 755 "$chroot_dir/var/vcap/bosh/bin/bosh-agent"

echo "Unmounting and converting back to VMDK..."
umount "$chroot_dir"

qemu-img convert -f raw -O vmdk -o subformat=streamOptimized "$raw_disk" "$vmdk"

echo "Repacking stemcell..."
pushd "$input_disk"
  tar czf "$input_stemcell/image" ./*
popd

pushd "$input_stemcell"
  sed -i -e "s/^version:.*/version: $VERSION/" stemcell.MF
  tar czf "$output_stemcell/stemcell.tgz" ./*
popd

cp "$output_stemcell/stemcell.tgz" "$output_path"
echo ""
echo "Done. Repacked stemcell: $output_path"
