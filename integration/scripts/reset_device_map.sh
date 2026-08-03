#!/usr/bin/env bash
# reset_device_map.sh detaches every leftover loop device and removes its backing file. Noble's
# loop-device semantics require care:
#
#   - A plain `losetup -d` is unreliable: the /var/log bind mount and /var/vcap/data both sit on
#     these loop devices and are torn down with lazy unmounts, so a device can be transiently busy
#     and `losetup -d` returns "success" as an autoclear-pending detach that hasn't happened yet.
#   - Noble auto-scans partitions on loop devices, spawning child partition devices (loopNpM) whose
#     udev events must settle before the parent will detach.
#
# So we `udevadm settle` and retry (up to 5 attempts, sleeping 0.2s) until `losetup <dev>` shows the
# device gone, and only then delete backing files -- and only those no longer attached, since
# deleting the backing file of a still-attached loop is exactly what produced the "(deleted)" zombie
# loop devices we saw leaking across specs and destabilizing the next agent bootstrap.
sudo udevadm settle
for dev in $(sudo losetup -a | cut -f1 -d:); do
  for _ in 1 2 3 4 5; do
    sudo udevadm settle
    out=$(sudo losetup "$dev" 2>/dev/null || true)
    if [ -z "$out" ]; then break; fi
    sudo losetup -d "$dev" || true
    sleep 0.2
  done
  out=$(sudo losetup "$dev" 2>/dev/null || true)
  if [ -n "$out" ]; then echo "ResetDeviceMap: loop device $dev still attached after retries" >&2; fi
done
for f in /virtualfs-*; do
  [ -e "$f" ] || continue
  sudo losetup -ln -O BACK-FILE 2>/dev/null | awk -v f="$f" '$1==f{found=1} END{exit !found}' || sudo rm -f "$f"
done
