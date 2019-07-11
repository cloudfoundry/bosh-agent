#!/usr/bin/env bash

set -e

export PATH=/usr/local/ruby/bin:/usr/local/go/bin:$PATH
chown -R bosh .
cd bosh-agent
su bosh -c "env PATH=$PATH GO111MODULE=on bin/test-unit"
