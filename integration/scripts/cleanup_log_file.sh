#!/usr/bin/env bash
# cleanup_log_file.sh truncates the agent log and, on systemd, rotates/vacuums the journal.
# The Go caller prepends "SM=<service manager>".
: "${SM:?}"
truncate -s 0 /var/vcap/bosh/log/current
if [ "$SM" = systemd ]; then
  # NOTE: --vacuum-time=0s doesn't actually work
  journalctl --flush --rotate --vacuum-time=1s
fi
