#!/usr/bin/env bash
# cleanup_data_dir.sh returns the VM's ephemeral state to baseline: stop monit (waiting until every
# process is unmonitored), unmount /tmp, tear down each ephemeral mount with the shared detach_mount
# helper, then recreate the emptied directories with their expected ownership and modes.
#
# The Go caller prepends "SM=..." and "BLOBSTORE_DIR=..." and concatenates detach_mount.sh in front
# of this file. The empty HUP handler (trap '' HUP) keeps the batch running to completion even if a
# fuser-kill inside detach_mount takes down the SSH session (see detach_mount.sh). The monit-summary
# wait polls up to 3 times a second apart; "stopped" means every "Process" line reads "not
# monitored" (an empty summary trivially satisfies that).
#
# It also sweeps transient top-level uploads (logs.tgz, records.json, dummy_package.tgz,
# compiled-*.tgz) out of the blobstore assets dir so specs are isolated. `-maxdepth 1 -type f` leaves
# the read-only release/ fixtures (a subdirectory) intact, and the fake-blobstore binary and .ssh/
# live outside the assets dir so the sweep can't touch them. Fixtures staged by a spec's BeforeEach
# are re-copied each run, so sweeping them is safe.
: "${BLOBSTORE_DIR:?}"
trap '' HUP
sudo /var/vcap/bosh/bin/monit stop all || true
stopped=0
for _ in 1 2 3; do
  summary=$(sudo /var/vcap/bosh/bin/monit summary | grep 'Process' || true)
  total=$(printf '%s\n' "$summary" | grep -c 'Process' || true)
  notmon=$(printf '%s\n' "$summary" | grep -c 'not monitored' || true)
  if [ "$total" = "$notmon" ]; then stopped=1; break; fi
  sleep 1
done
if [ "$stopped" != 1 ]; then echo "ensureMonitStopped: monit processes not stopped" >&2; exit 1; fi
! mount | grep -q ' on /tmp ' || sudo umount /tmp
detach_mount /var/tmp
sudo rm -f /var/log/bosh-agent.log
detach_mount /var/log
detach_mount /opt
detach_mount /var/opt
detach_mount /var/vcap/data
sudo mkdir -p /var/tmp
sudo chmod 700 /var/tmp
sudo chmod 1777 /tmp
sudo mkdir -p /var/log
sudo chmod 775 /var/log
sudo chown root:syslog /var/log
sudo mkdir -p /var/opt
sudo chmod 775 /var/opt
sudo chown root:root /var/opt
sudo mkdir -p /opt
sudo chmod 775 /opt
sudo chown root:root /opt
if [ -d "$BLOBSTORE_DIR" ]; then
  find "$BLOBSTORE_DIR" -maxdepth 1 -type f -delete
fi
