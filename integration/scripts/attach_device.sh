#!/usr/bin/env bash
# attach_device.sh loops over the partitions server-side so the whole AttachDevice sequence is one
# SSH round-trip. For each partition it grabs a free loop device, creates and attaches a (sparse)
# backing file, reads the loop's major:minor straight from sysfs (/sys/block/<loop>/dev, decimal),
# and re-creates the block node at the expected path. It echoes one
# "MAP <deviceNum> <loop> <node> <major:minor>" line per partition for the Go wrapper to record.
#
# Parameters (DEV/NPARTS/SIZE/START) are prepended by the Go caller as bash assignments.
: "${DEV:?}"
: "${NPARTS:?}"
: "${SIZE:?}"
: "${START:?}"
n="$START"
for i in $(seq 0 "$NPARTS"); do
  if [ "$i" -eq 0 ]; then p="$DEV"; else p="${DEV}${i}"; fi
  sudo rm -rf "/virtualfs-$n"
  sudo truncate -s "${SIZE}M" "/virtualfs-$n"
  loop=$(sudo losetup --find --show "/virtualfs-$n")
  devnum=$(cat "/sys/block/$(basename "$loop")/dev")
  major=${devnum%%:*}
  minor=${devnum##*:}
  sudo rm -f "$p"
  sudo mknod "$p" b "$major" "$minor"
  echo "MAP $n $loop $p $major:$minor"
  n=$((n+1))
done
