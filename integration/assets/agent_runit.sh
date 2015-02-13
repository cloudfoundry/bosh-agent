#!/bin/bash
set -e

export PATH=/var/vcap/bosh/bin:$PATH
exec 2>&1

cd /var/vcap/bosh

exec nice -n -10 /var/vcap/bosh/bin/bosh-agent -P ubuntu -C /var/vcap/bosh/agent.json
