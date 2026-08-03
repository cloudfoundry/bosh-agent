#!/usr/bin/env bash
# detach_mount() tears down a single mount directory. It is shared by cleanup_data_dir.sh and
# detach_device.sh: the Go caller prepends "SM=<service manager>" (and concatenates the caller's
# body) before this file, so detach_mount is defined once and reused.
#
# For the given directory it stops rsyslog first when detaching /var/log on systemd (see the long
# note below), then for each mount under $d -- deepest first -- fuser-kills holders and unmounts
# (lazily for /var/log), and finally removes the directory.
#
# On systemd-based stemcells (e.g. resolute), rsyslog is intentionally left running for the whole
# test run so it can keep forwarding journald output into /var/log/bosh-agent.log, which
# /var/vcap/bosh/log/current is symlinked to. We stop it before the fuser-kill/unmount so it cleanly
# releases its open handles on /var/log instead of racing the lazy unmount. We deliberately do NOT
# restart it: /var/log is about to become a plain directory on the root disk again, not the
# /var/vcap/data/root_log bind mount rsyslog expects. The agent's own Bootstrap (SetupLogDir then
# SetupLoggingAndAuditing, see agent/bootstrap.go) re-establishes that bind mount and restarts
# logging, in that order, the next time StartAgent() runs.
#
# /var/log is unmounted lazily to prevent intermittent test failures: as of 2024-06-24 it is a bind
# mount of /var/vcap/data/root_log, and for reasons we don't fully understand `fuser -k` doesn't
# consistently terminate its holders in time for a plain umount. Because we later unmount
# /var/vcap/data, a lingering /var/log reference will eventually fail loudly there, so this is safe.
: "${SM:?}"
detach_mount() {
  d="$1"
  if [ "$d" = /var/log ] && [ "$SM" = systemd ]; then
    sudo systemctl stop syslog.socket rsyslog.service || true
  fi
  mps=$(sudo mount | grep "on $d" | cut -d' ' -f3 | awk '{ print length"\t"$0 }' | sort -rn | cut -f2- || true)
  for mp in $mps; do
    [ -n "$mp" ] || continue
    # fuser -km can kill sshd holders (wtmp/lastlog on /var/log); the trap '' HUP in the caller keeps
    # this reparented shell alive to finish the umount/rm. Its non-zero exit on empty mounts is expected.
    sudo fuser -km "$mp" || true
    if [ "$mp" = /var/log ]; then
      sudo umount --lazy "$mp" || true
    else
      sudo umount "$mp" || true
    fi
  done
  sudo rm -rf "$d"
}
